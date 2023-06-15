# This file contains the branches that are no longer active with respect to releng
# Branches here are required for the gpac bundle to work but it is not necessary to clutter the gromit
# config file or main.tf with these.
historical_branches = [
{ branch    = "release-1.7",
	reviewers = "0",
	convos    = "false",
	source_branch  = "master",
	required_tests = ["1.15","Go 1.16 tests"]}
]
