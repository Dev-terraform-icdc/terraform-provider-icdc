package icdc

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

type SecurityGroup struct {
	Id                 string         `json:"id,omitempty"`
	Name               string         `json:"name"`
	EmsRef             string         `json:"ems_ref"`
	SecurityGroupRules []SecurityRule `json:"firewall_rules,omitempty"`
}

type SecurityGroupCollection struct {
	Resources []SecurityGroup `json:"resources"`
}

type SecurityGroupRequest struct {
	Action string `json:"action"`
	Name   string `json:"name"`
	Id     string `json:"id,omitempty"`
}

type MiqTaskResults struct {
	Results []struct {
		TaskId   string `json:"task_id"`
		Success  bool   `json:"success"`
		TaskHref string `json:"task_href"`
		Message  string `json:"message"`
	} `json:"results"`
}

type MiqTask struct {
	Id      string `json:"id"`
	State   string `json:"state"`
	Status  string `json:"status"`
	Success string `json:"success"`
	Message string `json:"message"`
}

type EmsProviderCollection struct {
	Resources []struct {
		Id string `json:"id"`
	}
}

func fetchEmsId() (string, error) {

	requestUrl := "api/compute/v1/providers?expand=resources&filter[]=type=ManageIQ::Providers::Redhat::NetworkManager"

	responseBody, err := requestApi("GET", requestUrl, nil)

	if err != nil {
		return "", err
	}

	var parsedBody EmsProviderCollection

	err = responseBody.Decode(&parsedBody)

	if err != nil {
		return "", err
	}

	return parsedBody.Resources[0].Id, nil
}

func groupsListSnapshot() (map[string]string, error) {
	securityGroups, err := securityGroupList()

	if err != nil {
		return nil, err
	}

	snapshot := make(map[string]string)

	for _, s := range securityGroups {
		snapshot[s.Id] = s.Name
	}

	return snapshot, nil
}

func securityGroupList() ([]SecurityGroup, error) {
	requestUrl := "api/compute/v1/security_groups?expand=resources&attributes=firewall_rules"
	responseBody, err := requestApi("GET", requestUrl, nil)

	if err != nil {
		return nil, fmt.Errorf("can't fetch security group list: %s", err)
	}

	var securityGroupCollection SecurityGroupCollection

	err = responseBody.Decode(&securityGroupCollection)

	if err != nil {
		return nil, fmt.Errorf("can't decode security group list: %s", err)
	}

	return securityGroupCollection.Resources, nil
}

var errSecurityGroupNotFound = errors.New("security group not found")

func fetchSecurityGroup(id string) (SecurityGroup, error) {
	if id == "" {
		return SecurityGroup{}, fmt.Errorf("cannot fetch security group without an ID")
	}
	path := fmt.Sprintf("api/compute/v1/security_groups/%s?expand=resources&attributes=firewall_rules", url.PathEscape(id))
	response, err := requestApiResponse("GET", path, nil)
	if err != nil {
		return SecurityGroup{}, fmt.Errorf("can't fetch security group: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return SecurityGroup{}, fmt.Errorf("%w: %s", errSecurityGroupNotFound, id)
	}
	if response.StatusCode != http.StatusOK {
		return SecurityGroup{}, fmt.Errorf("can't fetch security group: HTTP %d", response.StatusCode)
	}
	var group SecurityGroup
	if err := json.NewDecoder(response.Body).Decode(&group); err != nil {
		return SecurityGroup{}, fmt.Errorf("can't decode security group: %w", err)
	}
	if group.Id != id || group.Name == "" || group.EmsRef == "" {
		return SecurityGroup{}, fmt.Errorf("invalid security group response: missing fields or mismatched ID")
	}
	return group, nil
}
