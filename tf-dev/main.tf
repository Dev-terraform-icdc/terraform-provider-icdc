terraform {
  required_providers {
    icdc = {
      source = "local.com/icdc-io/icdc"
      version = "1.0.1"
    }
  }
}

provider "icdc" {
    username = "ekaribek@ibagroup.kz"
    password = "ac4c1381"
    location = "icz"
    auth_group = "icdc.member"  
}

data "icdc_template" "centos" {
    name = "CentOS Stream"
    version = "9-250714"
}

resource "icdc_dns_zone" "zone1"{
    name = "tf-workshop.icdc.at.icz.icdc.io"
}
