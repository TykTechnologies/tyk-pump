package openstack

import (
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"

	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/qos/rules"
)

func dataSourceNetworkingQoSDSCPMarkingRuleV2() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceNetworkingQoSDSCPMarkingRuleV2Read,
		Schema: map[string]*schema.Schema{
			"region": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},

			"qos_policy_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"dscp_mark": {
				Type:     schema.TypeInt,
				Computed: true,
				Optional: true,
				ForceNew: false,
			},
		},
	}
}

func dataSourceNetworkingQoSDSCPMarkingRuleV2Read(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	networkingClient, err := config.networkingV2Client(GetRegion(d, config))
	if err != nil {
		return fmt.Errorf("Error creating OpenStack networking client: %s", err)
	}

	listOpts := rules.DSCPMarkingRulesListOpts{}

	if v, ok := d.GetOk("dscp_mark"); ok {
		listOpts.DSCPMark = v.(int)
	}

	qosPolicyID := d.Get("qos_policy_id").(string)

	pages, err := rules.ListDSCPMarkingRules(networkingClient, qosPolicyID, listOpts).AllPages()
	if err != nil {
		return fmt.Errorf("Unable to retrieve openstack_networking_qos_dscp_marking_rule_v2: %s", err)
	}

	allRules, err := rules.ExtractDSCPMarkingRules(pages)
	if err != nil {
		return fmt.Errorf("Unable to extract openstack_networking_qos_dscp_marking_rule_v2: %s", err)
	}

	if len(allRules) < 1 {
		return fmt.Errorf("Your query returned no openstack_networking_qos_dscp_marking_rule_v2. " +
			"Please change your search criteria and try again.")
	}

	if len(allRules) > 1 {
		return fmt.Errorf("Your query returned more than one openstack_networking_qos_dscp_marking_rule_v2." +
			" Please try a more specific search criteria")
	}

	rule := allRules[0]
	id := resourceNetworkingQoSRuleV2BuildID(qosPolicyID, rule.ID)

	log.Printf("[DEBUG] Retrieved openstack_networking_qos_dscp_marking_rule_v2 %s: %+v", id, rule)
	d.SetId(id)

	d.Set("qos_policy_id", qosPolicyID)
	d.Set("dscp_mark", rule.DSCPMark)
	d.Set("region", GetRegion(d, config))

	return nil
}
