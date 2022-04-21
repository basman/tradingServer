package servicePriceVariation

import (
	"github.com/shopspring/decimal"
	"log"
	"math/rand"
	"time"
	"tradingServer/entity"
	"tradingServer/storage"
)

const minimumUpdateIntervalSeconds = 0.2

type PriceMaker struct {
	assetName    string
	currentPrice decimal.Decimal
	startPrice   decimal.Decimal

	targetPrice    decimal.Decimal
	changeInterval float64
	lastChange     time.Time

	priceUpdates chan entity.MarketAsset
}

func NewPriceMaker(assetName string, startPrice decimal.Decimal, ev chan entity.MarketAsset) *PriceMaker {
	pm := &PriceMaker{
		assetName:    assetName,
		currentPrice: startPrice,
		startPrice:   startPrice,

		targetPrice: startPrice,
		lastChange:  time.Now(),

		priceUpdates: ev,
	}

	pm.generateTarget()

	return pm
}

// update progresses the price towards the current (secret) goal
func (pm *PriceMaker) update() {
	if pm.changeInterval == 0 {
		panic("price maker's change interval has not been initialized (call generateTarget() before update())")
	}
	stepAmount := time.Now().Sub(pm.lastChange).Seconds() / pm.changeInterval

	if pm.currentPrice.Sub(pm.targetPrice).Abs().LessThanOrEqual(decimal.NewFromFloat(stepAmount)) {
		log.Printf("PriceMaker %v update: %v\n", pm.assetName, pm.currentPrice.StringFixed(3))
		pm.generateTarget()
	}

	pm.step()
}

func (pm *PriceMaker) step() {
	delta := time.Now().Sub(pm.lastChange).Seconds() / pm.changeInterval
	deltaDec := decimal.NewFromFloat(delta)

	// prevent overshooting
	if deltaDec.GreaterThan(pm.currentPrice.Sub(pm.targetPrice).Abs()) {
		deltaDec = pm.currentPrice.Sub(pm.targetPrice).Abs()
	}

	// target is lower than current price: negate delta
	if pm.targetPrice.LessThan(pm.currentPrice) {
		deltaDec = deltaDec.Neg()
	}

	pm.currentPrice = pm.currentPrice.Add(deltaDec)
	//log.Printf("PriceMaker %v price step: %v\n", pm.assetName, pm.currentPrice.StringFixed(3))

	// store new price
	db := storage.GetDatabase()
	err := db.SetAssetPrice(pm.assetName, pm.currentPrice)
	if err != nil {
		log.Printf("store new asset price failed for asset %v: %v\n", pm.assetName, err)
	}

	pm.lastChange = time.Now()

	pm.priceUpdates <- entity.MarketAsset{
		Name:  pm.assetName,
		Price: pm.currentPrice,
		When:  time.Now(),
	}
}

func (pm *PriceMaker) generateTarget() {
	// pick new random price variation +-[0,startPrice/2 * 0.1]
	delta := decimal.NewFromFloat((rand.Float64() - 0.5) * 0.1)
	pm.targetPrice = pm.startPrice.Add(pm.startPrice.Mul(delta))

	// pick a random change interval (in seconds) from range [1,10]
	pm.changeInterval = 9*rand.Float64() + 1

	//deltaAbs := pm.startPrice.Mul(delta).Div(decimal.NewFromFloat(pm.changeInterval))
	//log.Printf("PriceMaker %v new target price %v (%.3fs; delta=%v/s)\n",
	//	pm.assetName,
	//	pm.targetPrice.StringFixed(3),
	//	pm.changeInterval,
	//	deltaAbs.StringFixed(3))
}

func (pm *PriceMaker) Run() {
	for {
		subInterval := pm.changeInterval / 100

		if subInterval < minimumUpdateIntervalSeconds {
			subInterval = minimumUpdateIntervalSeconds
		}

		time.Sleep(time.Duration(subInterval * float64(time.Second)))
		pm.update()
	}
}
