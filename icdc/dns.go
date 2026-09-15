package icdc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// readDnsList validates the envelope before a caller treats an absent item as deleted.
// A missing collection endpoint is an error, not proof that a resource was deleted.
func readDnsList(path string, result interface{}) error {
	response, err := requestApiResponse("GET", path, nil)
	if err != nil {
		return fmt.Errorf("error fetching DNS list: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("error fetching DNS list: HTTP %d", response.StatusCode)
	}
	var envelope struct {
		Status int             `json:"status"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("error decoding DNS list: %w", err)
	}
	if envelope.Status != http.StatusOK {
		return fmt.Errorf("error fetching DNS list: API status %d", envelope.Status)
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return fmt.Errorf("invalid DNS list: missing data array")
	}
	if err := json.Unmarshal(envelope.Data, result); err != nil {
		return fmt.Errorf("error decoding DNS list data: %w", err)
	}
	return nil
}

type DnsZone struct {
	Name string `json:"name"`
}

type AddDnsZone struct {
	DnsZone `json:"zone"`
}

type AddDnsZoneResponse struct {
	DnsZone `json:"data"`
}

type DnsRecord struct {
	Payload DnsRecordBody `json:"record"`
}

type DnsRecordBody struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	Data     string `json:"data"`
	Priority int    `json:"priority,omitempty"`
	Weight   int    `json:"weight,omitempty"`
	Port     int    `json:"port,omitempty"`
	Ttl      int    `json:"ttl"`
}

type DnsRecordDetails struct {
	Id       string `json:"id"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Ttl      int    `json:"ttl"`
	Group    string `json:"group"`
	Data     string `json:"data"`
	Priority int    `json:"priority,omitempty"`
	Weight   int    `json:"weight,omitempty"`
	Port     int    `json:"port,omitempty"`
}

type responseListDnsRecords struct {
	Data []DnsRecordDetails `json:"data"`
}

func (r *DnsRecord) setAdditionalFields(d *schema.ResourceData) {
	switch t := strings.ToLower(r.Payload.Type); t {
	case "mx":
		(*r).Payload.Priority = d.Get("priority").(int)
	case "srv":
		(*r).Payload.Priority = d.Get("priority").(int)
		(*r).Payload.Weight = d.Get("weight").(int)
		(*r).Payload.Port = d.Get("port").(int)
	}
}
