package icdc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"strings"
)

func resourceSecurityRule() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceSecurityRuleCreate,
		ReadContext:   resourceSecurityRuleRead,
		UpdateContext: resourceSecurityRuleUpdate,
		DeleteContext: resourceSecurityRuleDelete,
		Schema: map[string]*schema.Schema{
			"id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"ems_ref": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"direction": {
				Type:             schema.TypeString,
				Required:         true,
				ValidateDiagFunc: validation.ToDiagFunc(validation.StringInSlice([]string{"egress", "ingress"}, true)),
			},
			"port_range": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			"protocol": {
				Type:             schema.TypeString,
				Optional:         true,
				Default:          "",
				ValidateDiagFunc: validation.ToDiagFunc(validation.StringInSlice([]string{"", "icmp", "tcp", "udp"}, true)),
			},
			"network_protocol": {
				Type:             schema.TypeString,
				Optional:         true,
				Default:          "ipv4",
				ValidateDiagFunc: validation.ToDiagFunc(validation.StringInSlice([]string{"ipv4", "ipv6"}, true)),
			},
			"remote_group_id": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			"remote_ip_subnet": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			"group_id": {
				Type:     schema.TypeString,
				Required: true,
			},
		},
	}
}

func resourceSecurityRuleCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	defer ctx.Done()
	var diags diag.Diagnostics

	var rangeMin, rangeMax string

	ranges := strings.Split(d.Get("port_range").(string), "-")

	if len(ranges) == 1 {
		rangeMin = ranges[0]
	} else {
		rangeMin = ranges[0]
		rangeMax = ranges[1]
	}

	securityGroup, err := fetchSecurityGroup(d.Get("group_id").(string))

	if err != nil {
		return append(diags, diag.FromErr(err)...)
	}

	rule := SecurityRule{
		Action:          "add_firewall_rule",
		Direction:       d.Get("direction").(string),
		PortRangeMin:    rangeMin,
		PortRangeMax:    rangeMax,
		Protocol:        d.Get("protocol").(string),
		NetworkProtocol: d.Get("network_protocol").(string),
		RemoteGroupId:   d.Get("remote_group_id").(string),
		SourceIpRange:   d.Get("remote_ip_subnet").(string),
		SecurityGroupId: securityGroup.EmsRef,
	}

	requestBody, err := json.Marshal(rule)

	if err != nil {
		return append(diags, diag.FromErr(err)...)
	}

	existedRules, err := rulesListSnapshot(d.Get("group_id").(string))

	er := make(map[string]int)

	for ndx, r := range existedRules {
		er[r.Id] = ndx
	}

	if err != nil {
		return append(diags, diag.FromErr(err)...)
	}

	requestUrl := fmt.Sprintf("api/compute/v1/security_groups/%s", d.Get("group_id").(string))
	responseBody, err := requestApi("POST", requestUrl, bytes.NewBuffer(requestBody))

	var miqTask MiqTask
	err = responseBody.Decode(&miqTask)

	if err != nil {
		return append(diags, diag.FromErr(err)...)
	}

	if miqTask.Success != "true" {
		err = fmt.Errorf(miqTask.Message)
		return append(diags, diag.FromErr(err)...)
	}

	securityGroup, err = fetchSecurityGroup(d.Get("group_id").(string))

	if err != nil {
		return append(diags, diag.FromErr(err)...)
	}

	var nr SecurityRule

	for _, r := range securityGroup.SecurityGroupRules {
		_, ok := er[r.Id]

		if !ok {
			nr = r
			break
		}
	}
	err = d.Set("ems_ref", nr.EmsRef)

	if err != nil {
		return append(diags, diag.FromErr(err)...)
	}

	d.SetId(nr.Id)

	return nil
}

func resourceSecurityRuleUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	defer ctx.Done()
	var diags diag.Diagnostics

	err := fmt.Errorf("method does not supported")
	return append(diags, diag.FromErr(err)...)
}

func resourceSecurityRuleRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	securityGroup, err := fetchSecurityGroup(d.Get("group_id").(string))
	if errors.Is(err, errSecurityGroupNotFound) {
		d.SetId("")
		return nil
	}
	if err != nil {
		return append(diags, diag.FromErr(err)...)
	}

	// An omitted or null collection does not confirm that a rule was deleted.
	if securityGroup.SecurityGroupRules == nil {
		return diag.Errorf("invalid security group response: missing firewall_rules array")
	}
	for _, rule := range securityGroup.SecurityGroupRules {
		if rule.Id == "" {
			return diag.Errorf("invalid security group response: rule without ID")
		}
	}
	var securityRule SecurityRule
	for _, r := range securityGroup.SecurityGroupRules {
		if r.Id == d.Id() {
			securityRule = r
			break
		}
	}

	if securityRule.Id == "" {
		d.SetId("")
		return nil
	}

	direction := normalizeSecurityRuleDirection(securityRule.Direction)
	networkProtocol := strings.ToLower(securityRule.NetworkProtocol)
	if securityRule.EmsRef == "" || (direction != "ingress" && direction != "egress") || (networkProtocol != "ipv4" && networkProtocol != "ipv6") {
		return diag.Errorf("invalid security rule response: missing or invalid required fields")
	}
	err = d.Set("ems_ref", securityRule.EmsRef)
	if err != nil {
		return append(diags, diag.FromErr(err)...)
	}

	err = d.Set("direction", normalizeSecurityRuleDirection(securityRule.Direction))
	if err != nil {
		return append(diags, diag.FromErr(err)...)
	}

	err = d.Set("port_range", securityRulePortRange(securityRule))
	if err != nil {
		return append(diags, diag.FromErr(err)...)
	}

	err = d.Set("protocol", securityRuleProtocol(securityRule))
	if err != nil {
		return append(diags, diag.FromErr(err)...)
	}

	err = d.Set("network_protocol", strings.ToLower(securityRule.NetworkProtocol))
	if err != nil {
		return append(diags, diag.FromErr(err)...)
	}

	err = d.Set("remote_group_id", securityRuleRemoteGroupId(securityRule))
	if err != nil {
		return append(diags, diag.FromErr(err)...)
	}

	err = d.Set("remote_ip_subnet", securityRule.SourceIpRange)
	if err != nil {
		return append(diags, diag.FromErr(err)...)
	}

	return nil
}

func normalizeSecurityRuleDirection(direction string) string {
	switch strings.ToLower(direction) {
	case "inbound":
		return "ingress"
	case "outbound":
		return "egress"
	default:
		return strings.ToLower(direction)
	}
}

func securityRulePortRange(rule SecurityRule) string {
	rangeMin := securityRuleValueToString(rule.PortRangeMin)
	if rangeMin == "" {
		rangeMin = securityRuleValueToString(rule.Port)
	}

	rangeMax := securityRuleValueToString(rule.PortRangeMax)
	if rangeMax == "" {
		rangeMax = securityRuleValueToString(rule.EndPort)
	}

	if rangeMin == "" {
		return ""
	}

	if rangeMax == "" || rangeMax == rangeMin {
		return rangeMin
	}

	return fmt.Sprintf("%s-%s", rangeMin, rangeMax)
}

func securityRuleProtocol(rule SecurityRule) string {
	if rule.Protocol != "" {
		return strings.ToLower(rule.Protocol)
	}

	return strings.ToLower(rule.HostProtocol)
}

func securityRuleRemoteGroupId(rule SecurityRule) string {
	if rule.RemoteGroupId != "" {
		return rule.RemoteGroupId
	}

	return rule.SourceSecurityGroupId
}

func securityRuleValueToString(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		return fmt.Sprintf("%g", v)
	default:
		return fmt.Sprint(v)
	}
}

func resourceSecurityRuleDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	defer ctx.Done()
	var diags diag.Diagnostics

	r := SecurityRule{
		Action: "remove_firewall_rule",
		Id:     d.Get("ems_ref").(string),
	}

	fmt.Printf("[---DEBUG---] rule %+v", r)

	requestBody, err := json.Marshal(r)
	requestUrl := fmt.Sprintf("api/compute/v1/security_groups/%s", d.Get("group_id").(string))
	responseBody, err := requestApi("POST", requestUrl, bytes.NewBuffer(requestBody))

	var miqTask MiqTaskDelete
	err = responseBody.Decode(&miqTask)

	fmt.Printf("[---DEBUG---] miqTask result %+v", miqTask)

	if err != nil {
		return append(diags, diag.FromErr(err)...)
	}

	if !miqTask.Success {
		err = fmt.Errorf(miqTask.Message)
		return append(diags, diag.FromErr(err)...)
	}

	d.SetId("")
	return nil
}
