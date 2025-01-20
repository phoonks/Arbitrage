package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	cf "github.com/kahsengphoon/IoTeX/config"
	"github.com/machinebox/graphql"
	"github.com/urfave/cli/v2"
)

const (
	uniswapAPI         = ""
	pancakeSwapAPI     = ""
	bridgeFee          = 10.0
	tradeAmount        = 100
	slippageTolerance  = 0.01
	stakingPoolAddress = ""
)

type Token struct {
	Symbol  string  `json: "symbol"`
	Address string  `json: "address"`
	Price   float64 `json: "price"`
}

type TokenUni struct {
	ID             string `json:"id"`
	Symbol         string `json:"symbol"`
	Name           string `json:"name"`
	DerivedETH     string `json:"derivedETH"`
	TotalLiquidity string `json:"totalLiquidity"`
}

type ResponseData struct {
	Tokens []TokenUni `json:"tokens"`
	Bundle Bundle     `json:"bundle"`
}

type Bundle struct {
	EthPrice string `json:"ethPrice"`
}

func (h *HttpServer) StartListenerServer(c *cli.Context) error {
	h.mtx.Lock()
	defer h.mtx.Unlock()

	if h.isStarted {
		return errors.New("Server already started")
	}

	r := gin.New()
	r.Use(gin.Recovery())
	h.isStarted = true
	h.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", h.port),
		Handler: r,
	}

	// Run()
	FetchAllAssetPrices()
	if err := r.Run(fmt.Sprintf(":%v", cf.Enviroment().AppServerPort)); err != nil {
		return err
	}

	return nil
}

func Run() {
	// * fetch token prices
	uniswapToken, err := FetchTokenPrices(uniswapAPI)
	if err != nil {
		fmt.Printf("[ERR] Uniswap FetchTokenPrices: %+v \n", err)
	}
	pancakeSwapToken, err := FetchTokenPrices(pancakeSwapAPI)
	if err != nil {
		fmt.Printf("[ERR] PancakeSwap FetchTokenPrices: %+v \n", err)
	}

	// * find and execute arbitrage opportunities
	FindArbitrageOpportunities(uniswapToken, pancakeSwapToken)
}

func FetchTokenPrices(apiUrl string) (map[string]Token, error) {
	resp, err := http.Get(apiUrl)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tokens map[string]Token
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, err
	}
	return tokens, nil
}

func FindArbitrageOpportunities(uniswapTokens, pancakeSwapTokens map[string]Token) {
	for symbol, token := range uniswapTokens {
		pancakeToken, exist := pancakeSwapTokens[symbol]
		if !exist {
			continue // no found then skip
		}

		uniswapPrice := token.Price
		pancakePrice := pancakeToken.Price

		if pancakePrice > uniswapPrice {
			ExecuteArbitrage(symbol, uniswapPrice, pancakePrice, pancakeSwapTokens)
		}
	}
}

func ExecuteArbitrage(symbol string, uniswapPrice, pancakePrice float64, pancakeSwapTokens map[string]Token) {
	profit := (pancakePrice-uniswapPrice)*tradeAmount/uniswapPrice - bridgeFee
	if profit <= 0 {
		fmt.Printf("No profit opportunity for %s. \n", symbol)
		return
	}

	fmt.Printf("[Arbitrage Found] Buy %s on Uniswap ($%.2f). sell on PancakeSwap ($%.2f). Estimated profit: $%.2f \n", symbol, uniswapPrice, pancakePrice, profit)

	// simulate trade and bridge
	fmt.Printf("Buying %s on Uniswap... \n", symbol)
	time.Sleep(1 * time.Second)

	newPancakePrice, exists := CheckPriceRecheck(symbol, pancakeSwapTokens)

	if !exists || newPancakePrice < pancakePrice*(1-slippageTolerance) {
		fmt.Printf("Price dropped on PancakeSwap for %s. Canceling arbitrage. \n", symbol)
		return
	}

	// simulate bridge and selling
	fmt.Printf("Bridge %s to PancakeSwap...\n", symbol)
	time.Sleep(3 * time.Second)
	fmt.Printf("Selling %s on PancakeSwap at updated price: $%.2f\n", symbol, newPancakePrice)

	// if price drop after bridge then to staking it
	if newPancakePrice < uniswapPrice*(1-slippageTolerance) {
	}
	fmt.Printf("Staking token due to price drop for %s \n", symbol)
}

func CheckPriceRecheck(symbol string, pancakeSwapTokens map[string]Token) (float64, bool) {
	pancakeToken, exists := pancakeSwapTokens[symbol]
	if !exists {
		return 0, false
	}

	return pancakeToken.Price, true
}

func FetchAllAssetPrices() {
	// GraphQL client
	client := graphql.NewClient("https://gateway.thegraph.com/api/1d7b11d6afd5da531b437c984c55444a/subgraphs/id/5zvR82QoaXYFyDEKLZ9t6v9adgnptxYpKpSbxtgVENFV")

	// GraphQL query
	req := graphql.NewRequest(`
		query {
			tokens(first: 1000) {
				id
				symbol
				name
				derivedETH
				totalLiquidity
			}
			bundle(id: "1") {
				ethPrice
			}
		}
	`)

	// Response data structure
	var respData ResponseData

	// Execute the query
	ctx := context.Background()
	if err := client.Run(ctx, req, &respData); err != nil {
		log.Fatalf("Error fetching data: %v", err)
	}

	// Parse ETH price in USD
	ethPrice, err := strconv.ParseFloat(respData.Bundle.EthPrice, 64)
	if err != nil {
		log.Fatalf("Error parsing ETH price: %v", err)
	}

	// Print token prices
	fmt.Println("Token Prices (in USD):")
	for _, token := range respData.Tokens {
		// Parse derivedETH
		derivedETH, err := strconv.ParseFloat(token.DerivedETH, 64)
		if err != nil {
			log.Printf("Error parsing derivedETH for token %s: %v", token.Symbol, err)
			continue
		}

		// Calculate USD price
		priceInUSD := derivedETH * ethPrice
		fmt.Printf("- %s (%s): $%.2f\n", token.Name, token.Symbol, priceInUSD)
	}
}
