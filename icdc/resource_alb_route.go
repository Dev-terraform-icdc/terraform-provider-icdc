package icdc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceAlbRoute() *schema.Resource {
	return &schema.Resource{
		Create: resourceAlbRouteCreate,
		Read:   resourceAlbRouteRead,
		Update: resourceAlbRouteUpdate,
		Delete: resourceAlbRouteDelete,
		Schema: map[string]*schema.Schema{
			"id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"cloudgw_name": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "alb-default",
			},
			"hostname": {
				Type:     schema.TypeString,
				Required: true,
			},
			"path": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "/",
			},
			"target_port": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  80,
			},
			"ip_version": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  4,
			},
			"insecure": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "none",
			},
			"tls_termination": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"services": {
				Type:     schema.TypeList,
				Required: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"healthcheck": {
				Type:     schema.TypeSet,
				Optional: true,
				Default:  nil,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"interval": {
							Type:     schema.TypeInt,
							Optional: true,
							Default:  30,
						},
						"timeout": {
							Type:     schema.TypeInt,
							Optional: true,
							Default:  5,
						},
						"path": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "/",
						},
						"scheme": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "http",
						},
						"port": {
							Type:     schema.TypeInt,
							Optional: true,
						},
						"hostname": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"follow_redirects": {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  true,
						},
						"method": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "get",
						},
					},
				},
			},
		},
	}
}

func resourceAlbRouteCreate(d *schema.ResourceData, m interface{}) error {

	services := servicesByExtId(d.Get("services").([]interface{}))

	healthcheck := d.Get("healthcheck").(*schema.Set)
	hcList := healthcheck.List()

	hcEnabled := len(hcList) > 0
	hc := Healthcheck{}
	if hcEnabled {
		hc.assignParams(d)
	}

	routeBody := AlbRoute{
		Name:               d.Get("name").(string),
		Hostname:           d.Get("hostname").(string),
		Path:               d.Get("path").(string),
		TargetPort:         d.Get("target_port").(int),
		Insecure:           d.Get("insecure").(string),
		TlsTermination:     d.Get("tls_termination").(string),
		CloudGatewayId:     cloudGwIdByName(d.Get("cloudgw_name").(string)),
		IpVersion:          strconv.Itoa(d.Get("ip_version").(int)),
		Services:           services,
		HealthcheckEnabled: hcEnabled,
		Healthcheck:        hc,
	}

	payload, err := json.Marshal(routeBody)
	body := bytes.NewBuffer(payload)
	if err != nil {
		return fmt.Errorf("can't marshalling into json %+v", routeBody)
	}

	requestUrl := "api/traefik_manager/v1/routes"
	responseBody, err := requestApi("POST", requestUrl, body)

	if err != nil {
		return err
	}

	var route AlbRoute

	err = responseBody.Decode(&route)

	if err != nil {
		return err
	}

	d.SetId(strconv.Itoa(route.Id))

	return nil
}

func resourceAlbRouteRead(d *schema.ResourceData, m interface{}) error {
	response, err := requestApiResponse("GET", fmt.Sprintf("api/traefik_manager/v1/routes/%s", d.Id()), nil)
	if err != nil {
		return fmt.Errorf("error fetching alb route: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		d.SetId("")
		return nil
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("error fetching alb route: HTTP %d", response.StatusCode)
	}
	var result AlbRouteApi
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return fmt.Errorf("error decoding alb route: %w", err)
	}
	route := result.Route
	if route == nil || strconv.Itoa(route.Id) != d.Id() {
		return fmt.Errorf("invalid alb route response: missing or mismatched route ID")
	}
	ipVersion, err := strconv.Atoi(route.IpVersion.String())
	if err != nil || (ipVersion != 4 && ipVersion != 6) {
		return fmt.Errorf("invalid alb route IP version: %q", route.IpVersion)
	}
	if route.CloudGateway == nil || route.CloudGateway.Name == "" {
		return fmt.Errorf("invalid alb route response: missing cloud gateway name")
	}
	services := make([]string, 0, len(route.Services))
	remaining := make(map[string]bool)
	for _, service := range route.Services {
		if service.ExtId <= 0 {
			return fmt.Errorf("invalid alb service: missing ext_id")
		}
		remaining[strconv.Itoa(service.ExtId)] = true
	}
	// API ordering is not significant; retain the order of existing members.
	for _, current := range d.Get("services").([]interface{}) {
		id := current.(string)
		if remaining[id] {
			services = append(services, id)
			delete(remaining, id)
		}
	}
	for _, service := range route.Services {
		id := strconv.Itoa(service.ExtId)
		if remaining[id] {
			services = append(services, id)
			delete(remaining, id)
		}
	}
	healthcheck := []interface{}{}
	// For the API version tested here, observed responses return healthcheck=null.
	// TODO: When healthcheck data is returned, verify and extend the mapping below as needed.
	if route.HealthcheckEnabled {
		hc := route.Healthcheck
		if hc == nil {
			return fmt.Errorf("invalid alb route response: enabled healthcheck is missing")
		}
		healthcheck = append(healthcheck, map[string]interface{}{
			"path": hc.Path, "scheme": hc.Scheme, "hostname": hc.Hostname,
			"port": hc.Port, "interval": hc.Interval, "timeout": hc.Timeout,
			"follow_redirects": hc.FollowRedirects, "method": hc.Method,
		})
	}
	for key, value := range map[string]interface{}{
		"name": route.Name, "hostname": route.Hostname, "path": route.Path,
		"target_port": route.TargetPort, "insecure": route.Insecure,
		"tls_termination": route.TlsTermination, "cloudgw_name": route.CloudGateway.Name,
		"ip_version": ipVersion, "services": services, "healthcheck": healthcheck,
	} {
		if err := d.Set(key, value); err != nil {
			return fmt.Errorf("error setting alb route %s: %w", key, err)
		}
	}
	return nil
}

func resourceAlbRouteUpdate(d *schema.ResourceData, m interface{}) error {

	/* ahrechushkin: update action will be implemented after implementing PATCH action in alb-api
	var route AlbRoute

	if d.HasChange("name") {
		route.Name = d.Get("name").(string)
	}

	if d.HasChange("hostname") {
		route.Hostname = d.Get("hostname").(string)
	}

	if d.HasChange("path") {
		route.Path = d.Get("path").(string)
	}

	if d.HasChange("target_port") {
		route.TargetPort = d.Get("target_port").(int)
	}

	if d.HasChange("insecure") {
		route.Insecure = d.Get("insecure").(string)
	}

	if d.HasChange("tls_termination") {
		route.TlsTermination = d.Get("tls_termination").(string)
	}

	if d.HasChange("services") {
		route.Services = servicesByExtId(d.Get("services").([]interface{}))
	}

	if d.HasChange("ip_version") {
		route.IpVersion = strconv.Itoa(d.Get("ip_version").(int))
	}

	fmt.Printf("[---DEBUG---] route changes %+v", route)

	route.CloudGatewayId = cloudGwIdByName(d.Get("cloudgw_name").(string))
	if d.Get("healthcheck_enabled") != nil {
		hc := Healthcheck{}
		hc.assignParams(d)
		route.Healthcheck = hc
	}

	routeBody := AlbRouteApi{Route: route}

	payload, err := json.Marshal(routeBody)
	body := bytes.NewBuffer(payload)
	if err != nil {
		return fmt.Errorf("can't marshalling into json %+v", routeBody)
	}

	requestUrl := fmt.Sprintf("api/traefik_manager/v1/routes/%s", d.Id())
	_, err = requestApi("PUT", requestUrl, body)

	if err != nil {
		return err
	}
	*/

	return nil
}

func resourceAlbRouteDelete(d *schema.ResourceData, m interface{}) error {
	requestUrl := fmt.Sprintf("api/traefik_manager/v1/routes/%s", d.Id())
	_, err := requestApi("DELETE", requestUrl, nil)

	if err != nil {
		return fmt.Errorf("error deleting alb_route resource, %s", err)
	}

	d.SetId("")
	return nil
}
