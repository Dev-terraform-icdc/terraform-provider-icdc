package icdc

import (
	"encoding/json"
	"slices"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type CloudGateway struct {
	Id   int    `json:"id"`
	Uid  string `json:"cloudgw_id"`
	Name string `json:"name"`
}

type AlbRouteApi struct {
	Route *AlbRouteDetails `json:"route"`
}

type AlbRouteDetails struct {
	Id                 int           `json:"id"`
	Name               string        `json:"name"`
	Hostname           string        `json:"hostname"`
	Path               string        `json:"path"`
	TargetPort         int           `json:"target_port"`
	Insecure           string        `json:"insecure"`
	TlsTermination     string        `json:"tls_termination"`
	IpVersion          json.Number   `json:"ip_version"`
	Services           []AlbService  `json:"services"`
	HealthcheckEnabled bool          `json:"healthcheck_enabled"`
	CloudGateway       *CloudGateway `json:"cloud_gateway"`
	Healthcheck        *Healthcheck  `json:"healthcheck"`
}

// Accept numeric API responses while preserving the existing create payload.
func (r *AlbRoute) UnmarshalJSON(data []byte) error {
	type routeAlias AlbRoute
	var wire struct {
		routeAlias
		IpVersion json.Number `json:"ip_version"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*r = AlbRoute(wire.routeAlias)
	r.IpVersion = wire.IpVersion.String()
	return nil
}

type AlbRoute struct {
	Id                 int          `json:"id,omitempty"`
	Name               string       `json:"name"`
	Hostname           string       `json:"hostname"`
	Path               string       `json:"path"`
	TargetPort         int          `json:"target_port"`
	Insecure           string       `json:"insecure,omitempty"`
	TlsTermination     string       `json:"tls_termination,omitempty"`
	CloudGatewayId     int          `json:"cloud_gateway_id"`
	IpVersion          string       `json:"ip_version"`
	Services           []AlbService `json:"services"`
	HealthcheckEnabled bool         `json:"healthcheck_enabled"`
	Healthcheck        `json:"healthcheck,omitempty"`
}

type Healthcheck struct {
	Path            string `json:"path"`
	Scheme          string `json:"scheme"`
	Hostname        string `json:"hostname"`
	Port            int    `json:"port"`
	Interval        int    `json:"interval"`
	Timeout         int    `json:"timeout"`
	FollowRedirects bool   `json:"follow_redirects"`
	Method          string `json:"method"`
}

type AlbService struct {
	Id    int `json:"id,omitempty"`
	ExtId int `json:"ext_id"`
}

func cloudGwIdByName(n string) int {
	requestUrl := "api/traefik_manager/v1/gateways"
	responseBody, err := requestApi("GET", requestUrl, nil)

	if err != nil {
		panic(err)
	}

	var gatewaysList []CloudGateway
	err = responseBody.Decode(&gatewaysList)

	if err != nil {
		panic(err)
	}

	for _, gateway := range gatewaysList {
		if gateway.Name == n {
			return gateway.Id
		}
	}
	return 0
}

func servicesByExtId(eIds []interface{}) []AlbService {
	requestUrl := "api/traefik_manager/v1/services"
	responseBody, err := requestApi("GET", requestUrl, nil)

	if err != nil {
		panic(err)
	}

	var servicesList []AlbService
	err = responseBody.Decode(&servicesList)

	if err != nil {
		panic(err)
	}

	result := []AlbService{}
	ids := []string{}

	for _, eid := range eIds {
		ids = append(ids, eid.(string))
	}

	for _, service := range servicesList {
		if slices.Contains(ids, strconv.Itoa(service.ExtId)) {
			result = append(result, service)
		}
	}

	return result
}

func (hc *Healthcheck) assignParams(d *schema.ResourceData) {

	hcParams := d.Get("healthcheck").(*schema.Set)
	hcParamsList := hcParams.List()
	hcMap := hcParamsList[0].(map[string]interface{})

	hc.Port = hcMap["port"].(int)
	hc.Path = hcMap["path"].(string)
	hc.Hostname = hcMap["hostname"].(string)
	hc.FollowRedirects = hcMap["follow_redirects"].(bool)
	hc.Interval = hcMap["interval"].(int)
	hc.Timeout = hcMap["timeout"].(int)
	hc.Scheme = hcMap["scheme"].(string)
	hc.Method = hcMap["method"].(string)
}
