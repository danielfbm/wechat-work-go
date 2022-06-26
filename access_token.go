package wechatwork

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/resty.v1"
)

// getAccessTokenFromServer 获取 access token
//
// https://work.weixin.qq.com/api/doc#90000/90135/91039
func (app *App) getAccessTokenFromServer() (RespAccessToken, error) {
	apiPath := "/cgi-bin/gettoken"
	resty.SetHostURL("https://qyapi.weixin.qq.com")
	resty.SetQueryParam("corpid", app.CorpID)
	resty.SetQueryParam("corpsecret", app.CorpSecret)
	resty.SetDebug(os.Getenv("DEBUG") == "true")

	var result RespAccessToken
	resp, err := resty.R().SetResult(&result).Get(apiPath)
	if err != nil {
		// log.Println("err when request gettoken api")
		// fmt.Println(err)
		// fmt.Println(resp.Status())
		// fmt.Println(resp.StatusCode())
		// fmt.Printf("%+v\n", data)
		return RespAccessToken{}, err
	}

	// 40001 不合法的secret参数
	// 40013 不合法的CorpID
	// 40056 不合法的agentid
	// 前两种情况 直接panic就好了 无需retry
	// access_token是分应用的，获取access_token 无需AgentID
	// 但是操作某些api 需要传AgentID
	if result.ErrCode == 40013 {
		return RespAccessToken{}, fmt.Errorf("请检查CorpID 参数")
	}

	if result.ErrCode == 40001 {
		return RespAccessToken{}, fmt.Errorf("请检查下CorpSecret 参数是否设置或者正确, 提示: 如果你在管理工具>通讯录同步中重置过Secret, 请注意更换CONTACT_APP_SECRET env")
	}

	if len(resp.Body()) == 0 {
		return RespAccessToken{}, fmt.Errorf("在获取token的时候, 企业微信服务器的接口返回了空的Body, 这很腾讯, 通常如果corpid和corpsecret都错了(各删一个字符试试)的情况下会这样, 总之都检查一下")
	}

	// 全局错误码 https://work.weixin.qq.com/api/doc#90000/90139/90313
	// -1 表示系统繁忙
	if result.ErrCode == -1 { //nolint
		// TODO: retry logic here
	}

	accessTokenCacheKey := fmt.Sprintf("access_token_%d", app.AgentID)
	expires := result.ExpiresInSecs - 1500
	err = app.Cache.Set(accessTokenCacheKey, result.AccessToken, time.Duration(expires)*time.Second)
	if err != nil {
		return RespAccessToken{}, err
	}

	return result, nil
}

// GetAccessToken GetAccessToken()
//
//
func (app *App) GetAccessToken() string {
	app.accessTokenLock.Lock()
	defer app.accessTokenLock.Unlock()

	accessTokenCacheKey := fmt.Sprintf("access_token_%d", app.AgentID)
	val := app.Cache.Get(accessTokenCacheKey)
	if val != nil {
		return val.(string)
	}

	// 从微信服务器获取
	var result RespAccessToken
	result, err := app.getAccessTokenFromServer()
	if err != nil {
		panic(err)
	}
	return result.AccessToken
}

// // SyncAccessToken 同步该app实例的 access token
// func (app *App) SyncAccessToken() error {
// 	tok, err := app.GetAccessToken()
// 	if err != nil {
// 		fmt.Println(err)
// 		return err
// 	}

// 	app.tokenMu.Lock()
// 	defer app.tokenMu.Unlock()

// 	// app.AccessToken = tok.AccessToken
// 	// app.ExpiresIn = tok.ExpiresInSecs
// 	return nil
// }

// // AccessTokenRefresher Refresh ExpiresIn in Ticker
// func (app *App) AccessTokenRefresher(o *sync.Once) {
// 	o.Do(func() {
// 		if app.AccessToken == "" {
// 			app.SyncAccessToken()
// 		}
// 		// tickDuration := time.Minute * 1
// 		// ticker := time.NewTicker(tickDuration)
// 		// for {
// 		// 	select {
// 		// 	case <-ticker.C:
// 		// 		// fmt.Println(".........tock .....")
// 		// 		// fmt.Println("access_token", app.AccessToken)
// 		// 		// fmt.Println("expires_in", app.ExpiresIn)
// 		// 		if app.ExpiresIn <= 7140 {
// 		// 			fmt.Println("expires_in 剩余时间不多，重新获取access_token")
// 		// 			// 如果ExpiresIn < 10 分钟那就重新发起请求获取
// 		// 			app.SyncAccessToken()
// 		// 		} else {
// 		// 			app.ExpiresIn -= 60
// 		// 			fmt.Println("after reduce expires_in")
// 		// 			fmt.Println("access_token", app.AccessToken)
// 		// 			fmt.Println("expires_in", app.ExpiresIn)
// 		// 		}
// 		// 	}
// 		// }
// 	})
// }

// // SpawnAccessTokenRefresher 启动该 app 的 access token 刷新 goroutine
// func (app *App) SpawnAccessTokenRefresher() {
// 	once := &sync.Once{}
// 	go app.AccessTokenRefresher(once)
// }

// GetUserInfo 网页授权或扫码授权时 AccessToken 和 Code 获取 UserInfo
//
// https://work.weixin.qq.com/api/doc#90000/90135/91023
//
// https://work.weixin.qq.com/api/doc#90000/90135/91437
func (app *App) GetUserInfo(code string) (RespGetUserInfo, error) {
	apiPath := "/cgi-bin/user/getuserinfo"
	uri := fmt.Sprintf("%s?access_token=%s&code=%s", apiPath, app.GetAccessToken(), code)
	var result RespGetUserInfo
	err := app.SimpleGet(uri, &result)
	if err != nil {
		return RespGetUserInfo{}, err
	}
	return result, nil
}

type RespAccessToken struct {
	RespCommon

	AccessToken   string `json:"access_token"`
	ExpiresInSecs int    `json:"expires_in"`
}

type RespGetUserInfo struct {
	RespCommon

	UserID   string `json:"UserId"`
	OpenID   string `json:"OpenId"`
	DeviceID string `json:"DeviceId"`
}
