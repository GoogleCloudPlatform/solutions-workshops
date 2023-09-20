# Tags

Tag names MUST follow the format `[component]/v[version]`, e.g.,
`grpc-xds/v1.0.0`. The `component` MUST start with a lowercase letter and MUST
only include lowercase letters, numbers, `-`, and `_`. The `component` name
SHOULD match either a top-level directory in the repository, or the scope
label used in [commit messages](commits.md#commit-messages).

The `version` MUST follow
[Semantic Versioning (SemVer) formatting](https://semver.org/#backusnaur-form-grammar-for-valid-semver-versions).
The `version` SHOULD also follow the
[Semantic Versioning (SemVer) specification](https://semver.org/).

In the GitHub repository, the tag name format is enforced on pushes
by a
[tag ruleset](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/creating-rulesets-for-a-repository),
using
[metadata restrictions](https://docs.github.com/en/enterprise-cloud@latest/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/available-rules-for-rulesets#metadata-restrictions)
with a
[regular expression](https://docs.github.com/en/enterprise-cloud@latest/organizations/managing-organization-settings/creating-rulesets-for-repositories-in-your-organization#about-regular-expressions-for-commit-metadata).
