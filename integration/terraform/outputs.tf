# Generated by: tyk-ci/wf-gen
# Generated on: Tue  2 Mar 12:26:37 UTC 2021

# Generation commands:
# ./pr.zsh -title Bare word -branch goreleaser/fixes -base goreleaser/fixes -repos tyk,tyk-analytics,tyk-pump,tyk-sink -p
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
