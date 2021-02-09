variable "aws_access_key" {
  type      = string
  default   = "${env("AWS_ACCESS_KEY_ID")}"
  sensitive = true
}

variable "aws_secret_key" {
  type      = string
  default   = "${env("AWS_SECRET_ACCESS_KEY")}"
  sensitive = true
}

variable "destination_regions" {
  type    = string
  default = "${env("DESTINATION_REGIONS")}"
}

variable "region" {
  type    = string
  default = "us-east-1"
}

variable "tyk_pump_version" {
  type    = string
  default = "${env("TYK_PUMP_VERSION")}"
}

# "timestamp" template function replacement
locals { timestamp = regex_replace(timestamp(), "[- TZ:]", "") }

source "amazon-ebs" "Pump" {
  access_key            = "{{user `aws_access_key`}}"
  ami_name              = "Tyk BYOL Pump {{user `tyk_pump_version`}} ({{user `flavour`}}) {{isotime | clean_resource_name}}"
  ami_regions           = "{{ user `destination_regions` }}"
  ena_support           = true
  force_delete_snapshot = true
  force_deregister      = true
  instance_type         = "t3.micro"
  region                = "{{user `region`}}"
  secret_key            = "{{user `aws_secret_key`}}"
  source_ami_filter {
    filters = {
      architecture                       = "x86_64"
      "block-device-mapping.volume-type" = "gp2"
      name                               = "{{user `ami_search_string`}}"
      root-device-type                   = "ebs"
      sriov-net-support                  = "simple"
      virtualization-type                = "hvm"
    }
    most_recent = true
    owners      = ["{{user `source_ami_owner`}}"]
  }
  sriov_support = true
  ssh_username  = "ec2-user"
  subnet_filter {
    filters = {
      "tag:Class" = "build"
    }
    most_free = true
    random    = false
  }
  tags = {
    Component = "pump"
    Flavour   = "{{user `flavour`}}"
    Product   = "byol"
    Version   = "{{user `tyk_pump_version`}}"
  }
}

# a build block invokes sources and runs provisioning steps on them. The
# documentation for build blocks can be found here:
# https://www.packer.io/docs/from-1.5/blocks/build
build {
  sources = ["source.amazon-ebs.Pump"]

  provisioner "shell" {
    environment_vars = ["TYK_PUMP_VERSION=${var.tyk_pump_version}"]
    script           = "install-pump.sh"
  }
}
