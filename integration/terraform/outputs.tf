# Generated by: tyk-ci/wf-gen
# Generated on: Thu 25 Feb 10:42:32 UTC 2021

# Generation commands:
# ./pr.zsh -title package name tyk-sink -branch goreleaser/all-tags
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
