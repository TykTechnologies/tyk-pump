# Generated by: tyk-ci/wf-gen
# Generated on: Fri  2 Jul 06:25:46 UTC 2021

# Generation commands:
# ./pr.zsh -title smoke tests -branch sync/releng1.4 -base sync/releng1.4 -repos tyk-pump -p
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
