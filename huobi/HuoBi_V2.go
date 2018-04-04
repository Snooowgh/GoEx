package huobi

import (
	"encoding/json"
	"errors"
	"fmt"
	. "github.com/Snooowgh/GoEx"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HuoBi_V2 struct {
	httpClient *http.Client
	accountId,
	baseUrl,
	accessKey,
	secretKey string
	marginAccountId map[string]string
}

type response struct {
	Status  string          `json:"status"`
	Data    json.RawMessage `json:"data"`
	Errmsg  string          `json:"err-msg"`
	Errcode string          `json:"err-code"`
}

func NewV2(httpClient *http.Client, accessKey, secretKey string) *HuoBi_V2 {
	hb2 := HuoBi_V2{httpClient:httpClient, baseUrl:"https://api.huobi.pro",accessKey:accessKey, secretKey:secretKey}
	hb2.GetAccountId()
	return &hb2
}
// [map[id:440177 type:spot subtype: state:working] map[id:2.741854e+06 type:margin subtype:bchusdt state:working] map[id:822037 type:margin subtype:btcusdt state:working] map[state:working id:1.214779e+06 type:margin subtype:eosusdt] map[type:margin subtype:ethusdt state:working id:2.137812e+06] map[id:2.741856e+06 type:margin subtype:iostusdt state:working] map[subtype:neousdt state:working id:2.741813e+06 type:margin] map[state:working id:2.196317e+06 type:margin subtype:qtumbtc] map[id:2.196318e+06 type:margin subtype:xrpbtc state:working] map[state:working id:1.44542e+06 type:margin subtype:xrpusdt] map[type:margin subtype:zecusdt state:working id:1.635943e+06] map[id:703716 type:otc subtype: state:working] map[id:2.969739e+06 type:point subtype: state:working]]
func (hbV2 *HuoBi_V2) GetAccountId() (string, error) {
	path := "/v1/account/accounts"
	params := &url.Values{}
	hbV2.buildPostForm("GET", path, params)

	//log.Println(hbV2.baseUrl + path + "?" + params.Encode())

	respmap, err := HttpGet(hbV2.httpClient, hbV2.baseUrl+path+"?"+params.Encode())
	if err != nil {
		return "", err
	}
	//log.Println(respmap)
	if respmap["status"].(string) != "ok" {
		return "", errors.New(respmap["err-code"].(string))
	}

	data := respmap["data"].([]interface{})
	accountIdMap := data[0].(map[string]interface{})
	hbV2.marginAccountId = make(map[string]string)
	for i,v := range data{
		if i==0{
			continue
		}else{
			temp := v.(map[string]interface{})
			hbV2.marginAccountId[temp["subtype"].(string)] = fmt.Sprintf("%.f", temp["id"].(float64))
		}
	}

	hbV2.accountId = fmt.Sprintf("%.f", accountIdMap["id"].(float64))
	
	//log.Println(respmap)
	return hbV2.accountId, nil
}

func (hbV2 *HuoBi_V2) GetAccount() (*Account, error) {
	path := fmt.Sprintf("/v1/account/accounts/%s/balance", hbV2.accountId)
	params := &url.Values{}
	params.Set("accountId-id", hbV2.accountId)
	hbV2.buildPostForm("GET", path, params)

	urlStr := hbV2.baseUrl + path + "?" + params.Encode()
	//println(urlStr)
	respmap, err := HttpGet(hbV2.httpClient, urlStr)

	if err != nil {
		return nil, err
	}

	//log.Println(respmap)

	if respmap["status"].(string) != "ok" {
		return nil, errors.New(respmap["err-code"].(string)+respmap["err-msg"].(string))
	}

	datamap := respmap["data"].(map[string]interface{})
	if datamap["state"].(string) != "working" {
		return nil, errors.New(datamap["state"].(string))
	}

	list := datamap["list"].([]interface{})
	acc := new(Account)
	acc.SubAccounts = make(map[Currency]SubAccount, 6)
	acc.Exchange = hbV2.GetExchangeName()

	subAccMap := make(map[Currency]*SubAccount)

	for _, v := range list {
		balancemap := v.(map[string]interface{})
		currencySymbol := balancemap["currency"].(string)
		currency := NewCurrency(currencySymbol, "")
		typeStr := balancemap["type"].(string)
		balance := ToFloat64(balancemap["balance"])
		if subAccMap[currency] == nil {
			subAccMap[currency] = new(SubAccount)
		}
		subAccMap[currency].Currency = currency
		switch typeStr {
		case "trade":
			subAccMap[currency].Amount = balance
		case "frozen":
			subAccMap[currency].ForzenAmount = balance
		}
	}

	for k, v := range subAccMap {
		acc.SubAccounts[k] = *v
	}

	return acc, nil
}

func (hbV2 *HuoBi_V2) GetRealtimePrice(pair CurrencyPair) (float64,float64){
	orders1, err := hbV2.GetTicker(pair)
	if err != nil {
		panic(err)
	}
	return orders1.Buy,orders1.Sell
}


func (hbV2 *HuoBi_V2) placeOrder(amount, price string, pair CurrencyPair, orderType string) (string, error) {
	path := "/v1/order/orders/place"
	params := url.Values{}
	params.Set("account-id", hbV2.accountId)
	params.Set("amount", amount)
	params.Set("symbol", strings.ToLower(pair.ToSymbol("")))
	params.Set("type", orderType)
	
	switch orderType {
	case "buy-limit", "sell-limit":
		params.Set("price", price)
	}

	hbV2.buildPostForm("POST", path, &params)

	resp, err := HttpPostForm3(hbV2.httpClient, hbV2.baseUrl+path+"?"+params.Encode(), hbV2.toJson(params),
		map[string]string{"Content-Type": "application/json", "Accept-Language": "zh-cn"})
	if err != nil {
		return "", err
	}

	respmap := make(map[string]interface{})
	err = json.Unmarshal(resp, &respmap)
	if err != nil {
		return "", err
	}

	if respmap["status"].(string) != "ok" {
		return "", errors.New(respmap["err-code"].(string))
	}

	return respmap["data"].(string), nil
}

func (hbV2 *HuoBi_V2) placeMarginkOrder(amount, price string, pair CurrencyPair, orderType string) (string, error) {
	path := "/v1/order/orders/place"
	params := url.Values{}
	p := strings.ToLower(pair.ToSymbol(""))
	id := hbV2.marginAccountId[p]
	if id != ""{
		params.Set("account-id", id)
	}else{
		panic(errors.New("Unsupported pairs!"))
	}
	params.Set("amount", amount)
	params.Set("symbol", strings.ToLower(pair.ToSymbol("")))
	params.Set("type", orderType)
	params.Set("source", "margin-api")
	
	switch orderType {
	case "buy-limit", "sell-limit":
		params.Set("price", price)
	}

	hbV2.buildPostForm("POST", path, &params)

	resp, err := HttpPostForm3(hbV2.httpClient, hbV2.baseUrl+path+"?"+params.Encode(), hbV2.toJson(params),
		map[string]string{"Content-Type": "application/json", "Accept-Language": "zh-cn"})
	if err != nil {
		return "", err
	}

	respmap := make(map[string]interface{})
	err = json.Unmarshal(resp, &respmap)
	if err != nil {
		return "", err
	}

	if respmap["status"].(string) != "ok" {
		return "", errors.New(respmap["err-code"].(string))
	}

	return respmap["data"].(string), nil
}

func (hbV2 *HuoBi_V2) LimitBuy(amount, price string, currency CurrencyPair) (*Order, error) {
	orderId, err := hbV2.placeOrder(amount, price, currency, "buy-limit")
	if err != nil {
		return nil, err
	}
	return &Order{
		Currency: currency,
		OrderID:  ToInt(orderId),
		Amount:   ToFloat64(amount),
		Price:    ToFloat64(price),
		Side:     BUY}, nil
}

func (hbV2 *HuoBi_V2) LimitSell(amount, price string, currency CurrencyPair) (*Order, error) {
	orderId, err := hbV2.placeOrder(amount, price, currency, "sell-limit")
	if err != nil {
		return nil, err
	}
	return &Order{
		Currency: currency,
		OrderID:  ToInt(orderId),
		Amount:   ToFloat64(amount),
		Price:    ToFloat64(price),
		Side:     SELL}, nil
}

func (hbV2 *HuoBi_V2) MarketBuy(amount, price string, currency CurrencyPair) (*Order, error) {
	orderId, err := hbV2.placeOrder(amount, price, currency, "buy-market")
	if err != nil {
		return nil, err
	}
	return &Order{
		Currency: currency,
		OrderID:  ToInt(orderId),
		Amount:   ToFloat64(amount),
		Price:    ToFloat64(price),
		Side:     BUY_MARKET}, nil
}

func (hbV2 *HuoBi_V2) MarketSell(amount, price string, currency CurrencyPair) (*Order, error) {
	orderId, err := hbV2.placeOrder(amount, price, currency, "sell-market")
	if err != nil {
		return nil, err
	}
	return &Order{
		Currency: currency,
		OrderID:  ToInt(orderId),
		Amount:   ToFloat64(amount),
		Price:    ToFloat64(price),
		Side:     SELL_MARKET}, nil
}


func (hbV2 *HuoBi_V2) LimitMarginBuy(amount, price string, currency CurrencyPair) (*Order, error) {
	orderId, err := hbV2.placeMarginkOrder(amount, price, currency, "buy-limit")
	if err != nil {
		return nil, err
	}
	return &Order{
		Currency: currency,
		OrderID:  ToInt(orderId),
		Amount:   ToFloat64(amount),
		Price:    ToFloat64(price),
		Side:     BUY}, nil
}

func (hbV2 *HuoBi_V2) LimitMarginSell(amount, price string, currency CurrencyPair) (*Order, error) {
	orderId, err := hbV2.placeMarginkOrder(amount, price, currency, "sell-limit")
	if err != nil {
		return nil, err
	}
	return &Order{
		Currency: currency,
		OrderID:  ToInt(orderId),
		Amount:   ToFloat64(amount),
		Price:    ToFloat64(price),
		Side:     SELL}, nil
}

func (hbV2 *HuoBi_V2) MarketMarginBuy(amount, price string, currency CurrencyPair) (*Order, error) {
	orderId, err := hbV2.placeMarginkOrder(amount, price, currency, "buy-market")
	if err != nil {
		return nil, err
	}
	return &Order{
		Currency: currency,
		OrderID:  ToInt(orderId),
		Amount:   ToFloat64(amount),
		Price:    ToFloat64(price),
		Side:     BUY_MARKET}, nil
}

func (hbV2 *HuoBi_V2) MarketMarginSell(amount, price string, currency CurrencyPair) (*Order, error) {
	orderId, err := hbV2.placeMarginkOrder(amount, price, currency, "sell-market")
	if err != nil {
		return nil, err
	}
	return &Order{
		Currency: currency,
		OrderID:  ToInt(orderId),
		Amount:   ToFloat64(amount),
		Price:    ToFloat64(price),
		Side:     SELL_MARKET}, nil
}

func (hbV2 *HuoBi_V2) parseOrder(ordmap map[string]interface{}) Order {
	ord := Order{
		OrderID:    ToInt(ordmap["id"]),
		Amount:     ToFloat64(ordmap["amount"]),
		Price:      ToFloat64(ordmap["price"]),
		DealAmount: ToFloat64(ordmap["field-amount"]),
		Fee:        ToFloat64(ordmap["field-fees"]),
		OrderTime:  ToInt(ordmap["created-at"]),
	}

	state := ordmap["state"].(string)
	switch state {
	case "submitted":
		ord.Status = ORDER_UNFINISH
	case "filled":
		ord.Status = ORDER_FINISH
	case "partial-filled":
		ord.Status = ORDER_PART_FINISH
	case "canceled", "partial-canceled":
		ord.Status = ORDER_CANCEL
	default:
		ord.Status = ORDER_UNFINISH
	}

	if ord.DealAmount > 0.0 {
		ord.AvgPrice = ToFloat64(ordmap["field-cash-amount"]) / ord.DealAmount
	}

	typeS := ordmap["type"].(string)
	switch typeS {
	case "buy-limit":
		ord.Side = BUY
	case "buy-market":
		ord.Side = BUY_MARKET
	case "sell-limit":
		ord.Side = SELL
	case "sell-market":
		ord.Side = SELL_MARKET
	}
	return ord
}

func (hbV2 *HuoBi_V2) GetOneOrder(orderId string, currency CurrencyPair) (*Order, error) {
	path := "/v1/order/orders/" + orderId
	params := url.Values{}
	hbV2.buildPostForm("GET", path, &params)
	respmap, err := HttpGet(hbV2.httpClient, hbV2.baseUrl+path+"?"+params.Encode())
	if err != nil {
		return nil, err
	}

	if respmap["status"].(string) != "ok" {
		return nil, errors.New(respmap["err-code"].(string))
	}

	datamap := respmap["data"].(map[string]interface{})
	order := hbV2.parseOrder(datamap)
	order.Currency = currency
	//log.Println(respmap)
	return &order, nil
}

func (hbV2 *HuoBi_V2) GetUnfinishOrders(currency CurrencyPair) ([]Order, error) {
	return hbV2.getOrders(queryOrdersParams{
		pair:   currency,
		states: "pre-submitted,submitted,partial-filled",
		size:   100,
		//direct:""
	})
}

func (hbV2 *HuoBi_V2) CancelOrder(orderId string, currency CurrencyPair) (bool, error) {
	path := fmt.Sprintf("/v1/order/orders/%s/submitcancel", orderId)
	params := url.Values{}
	hbV2.buildPostForm("POST", path, &params)
	resp, err := HttpPostForm3(hbV2.httpClient, hbV2.baseUrl+path+"?"+params.Encode(), hbV2.toJson(params),
		map[string]string{"Content-Type": "application/json", "Accept-Language": "zh-cn"})
	if err != nil {
		return false, err
	}

	var respmap map[string]interface{}
	err = json.Unmarshal(resp, &respmap)
	if err != nil {
		return false, err
	}

	if respmap["status"].(string) != "ok" {
		return false, errors.New(string(resp))
	}

	return true, nil
}

func (hbV2 *HuoBi_V2) GetOrderHistorys(currency CurrencyPair, currentPage, pageSize int) ([]Order, error) {
	return hbV2.getOrders(queryOrdersParams{
		pair:   currency,
		size:   pageSize,
		states: "partial-canceled,filled",
		direct: "next",
	})
}

type queryOrdersParams struct {
	types,
	startDate,
	endDate,
	states,
	from,
	direct string
	size int
	pair CurrencyPair
}

func (hbV2 *HuoBi_V2) getOrders(queryparams queryOrdersParams) ([]Order, error) {
	path := "/v1/order/orders"
	params := url.Values{}
	params.Set("symbol", strings.ToLower(queryparams.pair.ToSymbol("")))
	params.Set("states", queryparams.states)

	if queryparams.direct != "" {
		params.Set("direct", queryparams.direct)
	}

	if queryparams.size > 0 {
		params.Set("size", fmt.Sprint(queryparams.size))
	}

	hbV2.buildPostForm("GET", path, &params)
	respmap, err := HttpGet(hbV2.httpClient, fmt.Sprintf("%s%s?%s", hbV2.baseUrl, path, params.Encode()))
	if err != nil {
		return nil, err
	}

	if respmap["status"].(string) != "ok" {
		return nil, errors.New(respmap["err-code"].(string))
	}

	datamap := respmap["data"].([]interface{})
	var orders []Order
	for _, v := range datamap {
		ordmap := v.(map[string]interface{})
		ord := hbV2.parseOrder(ordmap)
		ord.Currency = queryparams.pair
		orders = append(orders, ord)
	}

	return orders, nil
}

func (hbV2 *HuoBi_V2) GetExchangeName() string {
	return "huobi.com"
}

func (hbV2 *HuoBi_V2) GetSymbols(long_polling string) ([]interface{},error){
	path := "/v1/common/symbols"
	params := url.Values{}
	params.Set("long-polling", long_polling)
	hbV2.buildPostForm("GET", path, &params)
	respmap, err := HttpGet(hbV2.httpClient, fmt.Sprintf("%s%s?%s", hbV2.baseUrl, path, params.Encode()))
	if err != nil {
		return nil, err
	}

	if respmap["status"].(string) != "ok" {
		return nil, errors.New(respmap["err-code"].(string))
	}
	datamap := respmap["data"].([]interface{})
	return datamap,nil
}
//完善深度函数
func (hbV2 *HuoBi_V2) GetTicker(currencyPair CurrencyPair) (*Ticker, error) {
	url := hbV2.baseUrl + "/market/detail/merged?symbol=" + strings.ToLower(currencyPair.ToSymbol(""))
	respmap, err := HttpGet(hbV2.httpClient, url)
	if err != nil {
		return nil, err
	}

	if respmap["status"].(string) == "error" {
		return nil, errors.New(respmap["err-msg"].(string))
	}

	tickmap, ok := respmap["tick"].(map[string]interface{})
	if !ok {
		return nil, errors.New("tick assert error")
	}

	ticker := new(Ticker)
	ticker.Vol = ToFloat64(tickmap["amount"])
	ticker.Low = ToFloat64(tickmap["low"])
	ticker.High = ToFloat64(tickmap["high"])
	bid, isOk := tickmap["bid"].([]interface{})
	if isOk != true {
		return nil, errors.New("no bid")
	}
	ask, isOk := tickmap["ask"].([]interface{})
	if isOk != true {
		return nil, errors.New("no ask")
	}
	ticker.Buy = ToFloat64(bid[0])
	ticker.Sell = ToFloat64(ask[0])
	ticker.Last = ToFloat64(tickmap["close"])
	ticker.Date = ToUint64(respmap["ts"])

	return ticker, nil
}

func (hbV2 *HuoBi_V2) GetDepth(size int, currency CurrencyPair) (*Depth, error) {
	url := hbV2.baseUrl + "/market/depth?symbol=%s&type=step0"
	respmap, err := HttpGet(hbV2.httpClient, fmt.Sprintf(url, strings.ToLower(currency.ToSymbol(""))))
	if err != nil {
		return nil, err
	}

	if "ok" != respmap["status"].(string) {
		return nil, errors.New(respmap["err-msg"].(string))
	}

	tick, _ := respmap["tick"].(map[string]interface{})
	bids, _ := tick["bids"].([]interface{})
	asks, _ := tick["asks"].([]interface{})

	depth := new(Depth)
	_size := size
	for _, r := range asks {
		var dr DepthRecord
		rr := r.([]interface{})
		dr.Price = ToFloat64(rr[0])
		dr.Amount = ToFloat64(rr[1])
		depth.AskList = append(depth.AskList, dr)

		_size--
		if _size == 0 {
			break
		}
	}

	_size = size
	for _, r := range bids {
		var dr DepthRecord
		rr := r.([]interface{})
		dr.Price = ToFloat64(rr[0])
		dr.Amount = ToFloat64(rr[1])
		depth.BidList = append(depth.BidList, dr)

		_size--
		if _size == 0 {
			break
		}
	}

	return depth, nil
}
// 1min: {'status': 'ok', 'ts': 1521594657015, 'tick': {'id': 1521594600, 'count': 93, 'close': 8966.45, 'open': 8956.77, 'amount': 18.096073528193205, 'low': 8950.05, 'vol': 162161.74979831, 'high': 8966.6}, 'ch': 'market.btcusdt.kline.1min'}
func (hbV2 *HuoBi_V2) GetKlineNewestRecords(currency CurrencyPair, period string) (*Kline, error) {
	url := hbV2.baseUrl + "/market/kline?symbol=%s&period=%s"
	respmap, err := HttpGet(hbV2.httpClient, fmt.Sprintf(url, strings.ToLower(currency.ToSymbol("")),period))
	if err != nil {
		return nil, err
	}
	if respmap["status"].(string) != "ok" {
		return nil, errors.New(respmap["err-code"].(string))
	}
	datamap := respmap["tick"].(map[string]interface{})
	return &Kline{respmap["ts"].(int64),datamap["open"].(float64),datamap["close"].(float64),datamap["high"].(float64),datamap["low"].(float64),datamap["amount"].(float64)},nil
}
// type Kline struct {
// 	Timestamp int64
// 	Open,
// 	Close,
// 	High,
// 	Low,
// 	Vol float64
// }
// {'data': [{'open': 8079.0, 'count': 108814, 'high': 8250.0, 'amount': 20360.770238282636, 'close': 7943.89, 'id': 1522080000, 'vol': 161917170.80999702, 'low': 7747.54}, {'open': 8469.32, 'count': 107475, 'high': 8650.0, 'amount': 18753.829437026616, 'close': 8079.0, 'id': 1521993600, 'vol': 155600300.47142583, 'low': 8022.0}, {'open': 8949.99, 'count': 103718, 'high': 8970.46, 'amount': 14693.676496052474, 'close': 8469.32, 'id': 1521907200, 'vol': 125971152.92548507, 'low': 8360.0}, {'open': 8626.15, 'count': 98340, 'high': 9020.0, 'amount': 13462.370862211717, 'close': 8950.0, 'id': 1521820800, 'vol': 118718300.93558992, 'low': 8560.0}, {'open': 8612.56, 'count': 100205, 'high': 8757.55, 'amount': 15804.689718100044, 'close': 8625.0, 'id': 1521734400, 'vol': 134596237.5220338, 'low': 8277.05}], 'ts': 1522140262156, 'status': 'ok', 'ch': 'market.btcusdt.kline.1day'}
func (hbV2 *HuoBi_V2) GetKlineHistoryRecords(currency CurrencyPair, period string,size string) ([]Kline, error) {
	url := hbV2.baseUrl + "/market/history/kline?symbol=%s&size=%s&period=%s"
	respmap, err := HttpGet(hbV2.httpClient, fmt.Sprintf(url, strings.ToLower(currency.ToSymbol("")),size,period))
	
	if err != nil {
		return nil, err
	}
	if respmap["status"].(string) != "ok" {
		return nil, errors.New(respmap["err-code"].(string)+" "+respmap["err-msg"].(string))
	}
	data := respmap["data"].([]interface{})
	result := make([]Kline,0)
	for _,datamap := range data{
		datamap:=datamap.(map[string]interface{})
		result=append(result, Kline{0,datamap["open"].(float64),datamap["close"].(float64),datamap["high"].(float64),datamap["low"].(float64),datamap["amount"].(float64)})
	}
	return result,nil
}
//非个人，整个交易所的交易记录
func (hbV2 *HuoBi_V2) GetTrades(currencyPair CurrencyPair, since int64) ([]Trade, error) {
		panic("not")
}

func (hbV2 *HuoBi_V2) buildPostForm(reqMethod, path string, postForm *url.Values) error {
	postForm.Set("AccessKeyId", hbV2.accessKey)
	postForm.Set("SignatureMethod", "HmacSHA256")
	postForm.Set("SignatureVersion", "2")
	postForm.Set("Timestamp", time.Now().UTC().Format("2006-01-02T15:04:05"))
	domain := strings.Replace(hbV2.baseUrl, "https://", "", len(hbV2.baseUrl))
	payload := fmt.Sprintf("%s\n%s\n%s\n%s", reqMethod, domain, path, postForm.Encode())
	sign, _ := GetParamHmacSHA256Base64Sign(hbV2.secretKey, payload)
//    println("sign:"+sign)
    postForm.Set("Signature", sign)
	return nil
}

func (hbV2 *HuoBi_V2) toJson(params url.Values) string {
	parammap := make(map[string]string)
	for k, v := range params {
		parammap[k] = v[0]
	}
	jsonData, _ := json.Marshal(parammap)
	return string(jsonData)
}
