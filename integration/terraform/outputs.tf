# Generated by: tyk-ci/wf-gen
# Generated on: Tue 27 Apr 11:36:54 UTC 2021

# Generation commands:
# ./pr.zsh -title releng: [TD-309] multiarch manifests for CI -branch releng/ecr -repos tyk-pump
# m4 -E -DxREPO=tyk-pump


data "terraform_remote_state" "integration" {
  backend = "remote"

  config = {
    organization = "Tyk"
    workspaces = {
      name = "base-prod"
    }
  }
}

output "tyk-pump" {
  value = data.terraform_remote_state.integration.outputs.tyk-pump
  description = "ECR creds for tyk-pump repo"
}

output "region" {
  value = data.terraform_remote_state.integration.outputs.region
  description = "Region in which the env is running"
}
