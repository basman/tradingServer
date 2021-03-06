package server

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
	"tradingServer/entity"
	"tradingServer/serviceTrade"
	"tradingServer/storage"
)

type Server interface {
	Run()
	GetEventInputChannel() chan entity.MarketAsset
}

type server struct {
	db               *storage.Database
	router           *gin.Engine
	priceUpdates     chan entity.MarketAsset
	streamClients    []*streamClient
	registerWsClient chan *streamClient
	removeWsClient   chan *streamClient
	rateLimitState   requestRateLimit
}

type streamClient struct {
	ws *websocket.Conn
	sync.RWMutex
	events   chan entity.MarketAsset
	shutdown bool
}

func NewServer() *server {
	g := gin.New()

	// Disable Console Color, you don't need console color when writing the logs to file.
	gin.DisableConsoleColor()

	// Logging to a file.
	f, _ := os.Create("gin.log")
	gin.DefaultWriter = io.MultiWriter(f)

	g.SetTrustedProxies(nil)

	s := &server{
		db:               storage.GetDatabase(),
		router:           g,
		priceUpdates:     make(chan entity.MarketAsset),
		registerWsClient: make(chan *streamClient, 10),
		removeWsClient:   make(chan *streamClient, 10),
		rateLimitState:   requestRateLimit{},
	}

	s.routes()

	return s
}

func (s *server) Run() {
	go s.serveStreamClients()

	err := s.router.Run(":8002")
	if err != nil {
		log.Fatalf("server start failed: %v", err.Error())
	}
}

func (s *server) GetEventInputChannel() chan entity.MarketAsset {
	return s.priceUpdates
}

func (s *server) routes() {
	s.router.GET("/", s.accessLog(), s.rateLimit("index", 10), s.handleIndex())

	txProtected := s.router.Group("", s.accessLog(), s.dbTransaction())
	txProtected.GET("/rates", s.rateLimit("rates", 20), s.dbTransaction(), s.handleRates())

	authenticated := txProtected.Group("", s.authRequired(), s.rateLimit("auth", 100))
	authenticated.GET("/account", s.handleAccount(false))
	authenticated.GET("/accounts", s.handleAccount(true))
	authenticated.POST("/buy", s.handleBuy())
	authenticated.POST("/sell", s.handleSell())

	authenticated.GET("/rates/stream", s.handlePriceStream())
}

func (s *server) handleIndex() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("content", "text/html")
		c.Writer.Write([]byte(`<html>
<body>
	<h2>GET requests</h2>
	<ul>
	<li><a href="rates">GET assets and their rates</a></li>
	<li><a href="account">GET account</a> - show your account</li>
	<li><a href="accounts">GET accounts</a> - show all accounts</li>
	</ul>
	<p/>
	<h2>POST requests</h2>
	<h3>POST /buy</h3>
	A user can buy any amount of an asset as far as his balance allows from the market.
	<p>
	Example JSON input:<br/>
	<pre>
{
	"asset": "white_wool",
	"amount": 34.95
}
	</pre>

	<h3>POST /sell</h3>
	A user can sell any amount of an asset to the market as far as his account allows.
	<p>
	Example JSON input:<br/>
	<pre>
{
	"asset": "white_wool",
	"amount": 34.95
}
	</pre>

	<h2>Web sockets</h2>
	<h3>GET /rates/stream</h3>
	Offers continuous price updates sent over a websocket, avoiding polling.<br/>
	Each message sent to the client contains one price update for one asset.<br/>
	<p>
	<pre>
{
	"Name": "white_wool",
	"Price": 43.703
}
	</pre>
	<p>
	No request content to be sent. Just connect to this endpoint and receive realtime price updates.<br/>
	<b>Authentication is required</b>, preserving the server's precious resources for people we trust. 
	<p>
	<h4>Note on price accuracy</h4> 
	The stream's message timeliness is limited to best effort and might be delayed. There is no guarantee by the server that any transaction you initiate
	will use the last price you received. It might just as well be subject to a price update still to be transmitted.
	<h4>Note on rounding</h4>
	The server uses <a href="https://pkg.go.dev/github.com/shopspring/decimal">decimal number representation</a> internally for all amounts of money.
	These numbers are converted to float64 before they are being sent over the wire or stored in a database. This may
	lead to rounding errors for odd fractions. 
</body>
</html>`))
	}
}

func (s *server) handleRates() gin.HandlerFunc {
	return func(c *gin.Context) {
		assets, err := s.db.GetAssets()
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.IndentedJSON(http.StatusOK, assets)
	}
}

func (s *server) handleBuy() gin.HandlerFunc {
	return func(c *gin.Context) {
		buf, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Printf("could not read post body: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		var trans entity.Transaction
		if err = json.Unmarshal(buf, &trans); err != nil {
			log.Printf("read json transaction failed: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		if !trans.Amount.IsPositive() {
			c.AbortWithStatusJSON(http.StatusBadRequest, newUserError("amount must be positive"))
		}

		price, err := s.db.GetAssetPrice(trans.Asset)
		if err != nil {
			log.Printf("could not get current asset price for '%v': %v", trans.Asset, err)
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		login, ok := getLoginFromContext(c)
		if !ok {
			return
		}

		balance, ok := getBalanceFromContext(c)
		if !ok {
			return
		}

		if price.Mul(trans.Amount).GreaterThan(balance) {
			c.AbortWithStatusJSON(http.StatusBadRequest,
				newUserError("Not enough funds. You want to spend %v but only have %v.",
					price.Mul(trans.Amount), balance))
		}

		acc, err := s.db.GetAccount(login)
		if err != nil {
			log.Printf("could not get account for login %v: %v", login, err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		err = serviceTrade.BuyAsset(acc, trans.Asset, trans.Amount)
		if err != nil {
			log.Printf("buy transaction failed (%v): %v", trans, err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.IndentedJSON(http.StatusOK, acc)
	}
}

func (s *server) handleSell() gin.HandlerFunc {
	return func(c *gin.Context) {
		buf, err := io.ReadAll(c.Request.Body)
		if err != nil {
			log.Printf("could not read post body: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		var trans entity.Transaction
		if err = json.Unmarshal(buf, &trans); err != nil {
			log.Printf("read json transaction failed: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		if !trans.Amount.IsPositive() {
			c.AbortWithStatusJSON(http.StatusBadRequest, newUserError("amount must be positive"))
			return
		}

		login, ok := getLoginFromContext(c)
		if !ok {
			return
		}

		acc, err := s.db.GetAccount(login)
		if err != nil {
			log.Printf("get account for login '%v' failed: %v", login, err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		asset := acc.GetOrCreateUserAsset(trans.Asset)

		if asset.Amount.LessThan(trans.Amount) {
			c.AbortWithStatusJSON(http.StatusBadRequest,
				newUserError("you can not sell more of %v than you currently have (%v)",
					trans.Asset, asset.Amount))
			return
		}

		err = serviceTrade.SellAsset(acc, trans.Asset, trans.Amount)
		if err != nil {
			log.Printf("sell asset %v for login '%v' failed: %v", trans.Asset, login, err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.IndentedJSON(http.StatusOK, acc)
	}
}

func (s *server) handleAccount(showAll bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if showAll {
			accList, err := s.db.GetAccounts()
			if err != nil {
				log.Printf("GetAccount failed: %v", err)
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}

			c.IndentedJSON(http.StatusOK, accList)
		} else {
			login, ok := getLoginFromContext(c)
			if !ok {
				return
			}

			acc, err := s.db.GetAccount(login)
			if err != nil {
				log.Printf("GetAccount failed: %v", err)
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			if acc == nil {
				log.Printf("handleAccount: no account found for login %v", login)
				c.AbortWithStatus(http.StatusNoContent)
				return
			}

			c.IndentedJSON(http.StatusOK, acc)
		}
	}
}

func (s *server) handlePriceStream() gin.HandlerFunc {
	upgrader := websocket.Upgrader{
		HandshakeTimeout: 2 * time.Second,
		WriteBufferSize:  1024,
	}

	return func(c *gin.Context) {
		ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("websocket handshake failed: %v", err)
			c.AbortWithStatus(http.StatusSwitchingProtocols)
			return
		}
		defer ws.Close()

		wsClient := &streamClient{
			ws:     ws,
			events: make(chan entity.MarketAsset, 1),
		}

		go func() {
			// read from client to detect disconnects early. we don't expect any data from client.
			_, _, err := wsClient.ws.NextReader()
			if err != nil {
				log.Printf("websocket %v read failure detected. closing connection.", ws.RemoteAddr())
			} else {
				log.Printf("websocket %v received data unexpectedly. closing connection.", ws.RemoteAddr())
			}
			s.removeWsClient <- wsClient
		}()

		// register websocket client to receive price changes
		s.registerWsClient <- wsClient

		// send price changes
		for ev := range wsClient.events {
			err := wsClient.sendEvent(ev)

			if err != nil {
				log.Printf("client %v send over websocket failed: %v", wsClient.ws.RemoteAddr(), err)
				s.removeWsClient <- wsClient
				// stay in the loop to consume remaining events. serveStreamClients will close the channel and trigger shutdown.
			}
		}
	}
}

func (wsClient *streamClient) sendEvent(ev entity.MarketAsset) error {
	wsClient.Lock()
	defer wsClient.Unlock()

	// enforce fast client readout
	wsClient.ws.SetWriteDeadline(time.Now().Add(1 * time.Second))
	err := wsClient.ws.WriteJSON(ev)
	// reset write timeout
	wsClient.ws.SetWriteDeadline(time.Time{})

	return err
}

func (s *server) serveStreamClients() {
	for {
		select {
		case c := <-s.registerWsClient:
			s.streamClients = append(s.streamClients, c)

		case c := <-s.removeWsClient:
			if c.shutdown {
				break
			}

			c.shutdown = true
			close(c.events)

			i := 0
			var c2 *streamClient
			for i, c2 = range s.streamClients {
				if c == c2 {
					s.streamClients[i] = nil
					break
				}
			}

			s.streamClients = append(s.streamClients[:i], s.streamClients[i+1:]...)

		case ev := <-s.priceUpdates:
			for _, client := range s.streamClients {
				if client == nil {
					continue
				}

				if !client.shutdown {
					client.events <- ev
				}
			}
		}
	}
}

func getBalanceFromContext(c *gin.Context) (decimal.Decimal, bool) {
	balanceI, ok := c.Get("balance")
	if !ok {
		log.Println("no balance found")
		c.AbortWithStatus(http.StatusInternalServerError)
		return decimal.Zero, false
	}

	balance := balanceI.(decimal.Decimal)
	return balance, true
}

func getLoginFromContext(c *gin.Context) (string, bool) {
	loginI, ok := c.Get("login")
	if !ok {
		log.Println("no login found")
		c.AbortWithStatus(http.StatusInternalServerError)
		return "", false
	}
	login := loginI.(string)
	return login, true
}
