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
      version = "5.16.0"
    }
  }
}

provider "github" {
  owner = "TykTechnologies"
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
  release_branches     = [
{ branch    = "master",
	reviewers = "2",
	convos    = "false",
	required_tests = ["1.15","Go 1.16 tests"]},
{ branch    = "release-1.7",
	reviewers = "2",
	convos    = "false",
	required_tests = ["1.15","Go 1.16 tests"]},
]
}