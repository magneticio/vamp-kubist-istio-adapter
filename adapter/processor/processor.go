package processor

import (
	"fmt"
	"net/http"

	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/configurator"
	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/models"
)

const bufferSize = 1000

var LogInstanceChannel = make(chan *models.LogInstance, bufferSize)
var ExperimentLogs = make(map[string]*models.ExperimentLogs)

/*
Example of a real log instance:

TimeStamp:  2019-06-19T12:09:30.732103777Z
Severity:  info
url :  /cart?variant_id=1
user :  unknown
cookies :  ex-1_user=3c445470-721f-48d2-ad4a-02de5d19988c;ex-1=dest-1-9191-subset2;guest_token=ImF3SzNsTlMyaHVLLUVGT3lqZXFjRFExNTYwOTQ0ODY2MTI4Ig%3D%3D--327ef99d0408d7d2738b87379d80015
a760d8594;_vshop_session=cURvN3QwQnFicy9JWWZWNWkzcy9vWDBzd09WN3RHTkZsWUs0eEwvblIwdWxwMEVjODlOVGhGbGlNNFN0US9ZV3hDRkFBTTNic0cwMnZuSVdtNS9ZaVE4Z29neGpkbFIxNVV5OG4xU3pvN3RxR3JQSHNGQ2Z2NFc

yQ2lzUFdYc1d5RFpEQTVlemJmenQ4NUhWWE1tV1B2MDdkc0N4RlNpQjZ5eEJ0Y3l5VWlkREIvby9JV1N0cms4a1RzbnV6Z21sLS1Mc1lEdHJTaEk1UDQrNnJSdWhmT0l3PT0%3D--35e006358222b813f496eb7e3ab4136886f1d0d0
destination :  demo-app
latency :  36.749194ms
responseCode :  200
responseSize :  4405
source :  gw-1-gateway
*/

func Process() {
	for {
		logInstance := <-LogInstanceChannel
		fmt.Printf("logInstance: %v\n", logInstance)
		ProcessInstance(configurator.ExperimentConfigurations, logInstance)
	}
}

func ProcessInstance(
	experimentConfigurations map[string]*models.ExperimentConfiguration,
	logInstance *models.LogInstance) {

	header := http.Header{}
	header.Add("Cookie", logInstance.Cookie)
	request := http.Request{
		Header: header,
	}

	fmt.Println(request.Cookies())

	for _, cookie := range request.Cookies() {
		if experimentConf, ok := experimentConfigurations[cookie.Name]; ok {
			fmt.Printf("experimentConf: %v\n", experimentConf)
			experimentName := cookie.Name
			if logInstance.URL == experimentConf.TargetPath {
				fmt.Printf("target Matched \n")
				if _, ok2 := experimentConf.Subsets[cookie.Value]; ok2 {
					fmt.Printf("subset ok\n")
					subsetName := cookie.Value
					userCookieName := experimentName + "_user"
					if userCookie, cookieErr := request.Cookie(userCookieName); cookieErr == nil {
						userID := userCookie.Value
						if _, ok := ExperimentLogs[experimentName]; !ok {
							ExperimentLogs[experimentName] = &models.ExperimentLogs{
								SubsetLogs: map[string]*models.SubsetLogs{
									subsetName: &models.SubsetLogs{
										UserLogs: map[string]int{userID: 0},
									},
								},
							}
						}
						if _, ok := ExperimentLogs[experimentName].SubsetLogs[subsetName]; !ok {
							ExperimentLogs[experimentName].SubsetLogs[subsetName] =
								&models.SubsetLogs{
									UserLogs: map[string]int{userID: 0},
								}
						}
						ExperimentLogs[experimentName].SubsetLogs[subsetName].UserLogs[userCookie.Value]++
					} else {
						fmt.Printf("cookieErr: %v\n", cookieErr)
					}
				}
			}
			break
		}
	}
}
