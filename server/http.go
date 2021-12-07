package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"io"
	"log"
	"net/http"
	"tradingServer/entity"
	"tradingServer/serviceTrade"
	"tradingServer/storage"
)

type Server interface {
	Run()
}

type server struct {
	db     *storage.Database
	router *gin.Engine
}

func NewServer() *server {
	g := gin.New()

	/*
		// Disable Console Color, you don't need console color when writing the logs to file.
		gin.DisableConsoleColor()

		// Logging to a file.
		f, _ := os.Create("gin.log")
		gin.DefaultWriter = io.MultiWriter(f)
	*/

	g.SetTrustedProxies(nil)

	s := &server{
		db:     storage.GetDatabase(),
		router: g,
	}

	s.routes()

	return s
}

func (s *server) Run() {
	err := s.router.Run(":8002")
	if err != nil {
		log.Fatalf("server start failed: %v", err.Error())
	}
}

func (s *server) routes() {
	s.router.GET("/", s.handleIndex())

	txProtected := s.router.Group("", s.DbTransaction())
	txProtected.GET("/rates", s.DbTransaction(), s.handleRates())

	authenticated := txProtected.Group("", s.AuthRequired())
	authenticated.GET("/account", s.handleAccount(false))
	authenticated.GET("/accounts", s.handleAccount(true))
	authenticated.POST("/buy", s.handleBuy())
	authenticated.POST("/sell", s.handleSell())
}

func (s *server) handleIndex() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("content", "text/html")
		c.Writer.Write([]byte(`<html>
<body>
	<h2>GET requests</h2>
	<ul>
	<li><a href="rates">assets and their rates</a></li>
	<li><a href="account">account</a> - show your account</li>
	<li><a href="accounts">accounts</a> - show all accounts</li>
	</ul>
	<p/>
	<h2>POST requests</h2>
	<h3>/buy</h3>
	A user can buy any amount of an asset as far as his balance allows from the market.
	<p>
	Example JSON input:<br/>
	<pre>
{
	"asset": "white_wool",
	"amount": 34.95
}
	</pre>

	<h3>/sell</h3>
	A user can sell any amount of an asset to the market as far as his account allows.
	<p>
	Example JSON input:<br/>
	<pre>
{
	"asset": "white_wool",
	"amount": 34.95
}
	</pre>

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
			c.AbortWithError(http.StatusBadRequest, errors.New("amount must be positive"))
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
			c.AbortWithError(http.StatusBadRequest,
				fmt.Errorf("Not enough funds. You want to spend %v but only have %v.",
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
			c.AbortWithError(http.StatusBadRequest, errors.New("amount must be positive"))
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
			c.AbortWithError(http.StatusBadRequest,
				fmt.Errorf("you can not sell more of %v than you currently have (%v)",
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
