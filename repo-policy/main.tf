terraform {

  #Being used until TFCloud can be used
  backend "remote" {
    hostname     = "app.terraform.io"
    organization = "Tyk"
    workspaces {
      name = "repo-policy-tyk-pump"
    }
  }

  required_providers {
    github = {
      source  = "integrations/github"
    }
  }
}

provider "github" {
  owner = "TykTechnologies"
}

# Copypasta from modules/github-repos/variables.tf
# FIXME: Unmodularise the github-repos module
variable "historical_branches" {
  type = list(object({
    branch         = string           # Name of the branch
    source_branch  = optional(string) # Source of the branch, needed when creating it
    reviewers      = number           # Min number of reviews needed
    required_tests = list(string)     # Workflows that need to pass before merging
    convos         = bool             # Should conversations be resolved before merging

  }))
  description = "List of branches managed by terraform"
}

module "tyk-pump" {
  source               = "./modules/github-repos"
  repo                 = "tyk-pump"
  description          = "Tyk Analytics Pump to move analytics data from Redis to any supported back end (multiple back ends can be written to at once)."
  default_branch       = "master"
  topics                      = []
  visibility                  = "public"
  wiki                        = false
  vulnerability_alerts        = true
  squash_merge_commit_message = "COMMIT_MESSAGES"
  squash_merge_commit_title   = "COMMIT_OR_PR_TITLE"
  release_branches     = concat(var.historical_branches, [
{ branch    = "master",
	reviewers = "1",
	convos    = "false",
	required_tests = ["1.19-bullseye","Go 1.19 tests"]},
{ branch    = "release-1.8",
	reviewers = "0",
	convos    = "false",
	source_branch  = "master",
	required_tests = ["1.16","Go 1.16 tests"]},
])
}
