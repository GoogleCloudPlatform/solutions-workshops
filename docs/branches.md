# Branches

Branch names MUST follow the format `[component]/[feature]`, e.g.,
`grpc-xds/some-experiment`. Both of `component` _and_ `feature` MUST start
with a lowercase letter and MUST only include lowercase letters, numbers, `-`,
and `_`. The `component` name SHOULD match either a top-level directory name
in the repository, or the scope used in
[commit messages](commits.md#commit-messages).

The only exception is the default `main` branch.

In the GitHub repository, the branch name format is enforced on pushes
by a
[branch ruleset](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/creating-rulesets-for-a-repository),
using
[metadata restrictions](https://docs.github.com/en/enterprise-cloud@latest/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/available-rules-for-rulesets#metadata-restrictions)
with a
[regular expression](https://docs.github.com/en/enterprise-cloud@latest/organizations/managing-organization-settings/creating-rulesets-for-repositories-in-your-organization#about-regular-expressions-for-commit-metadata).
