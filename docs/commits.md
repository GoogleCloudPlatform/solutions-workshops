# Commit messages

This repository uses
[Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/).

Commits to the default `main` branch MUST include message titles that contain
a type that communicates the intent, (e.g., `fix`, `feat`, `docs`, etc.) _and_
a scope (e.g., `feat(some-component)`). The scope MUST start with a lowercase
letter and MUST only include lowercase letters, numbers, `-`, and `_`. The
scope SHOULD clearly and uniquely identify a top-level directory in the
repository. It is acceptable to use a short version or abbreviation of the
directory name, as long as the unique requirement holds.

In the GitHub repository, the message title format is enforced on pushes
by a
[branch ruleset](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/creating-rulesets-for-a-repository),
using
[metadata restrictions](https://docs.github.com/en/enterprise-cloud@latest/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/available-rules-for-rulesets#metadata-restrictions)
with a
[regular expression](https://docs.github.com/en/enterprise-cloud@latest/organizations/managing-organization-settings/creating-rulesets-for-repositories-in-your-organization#about-regular-expressions-for-commit-metadata).

Locally, you can use the [`commit-msg`](../.git/hooks/commit-msg) Git hook to
verify message titles at commit time.

Plugins or extensions for IDEs are available to assist in writing commits:

- JetBrains IDEs:
  [Conventional Commit](https://plugins.jetbrains.com/plugin/13389-conventional-commit)
- VS Code:
  [Conventional Commits](https://marketplace.visualstudio.com/items?itemName=vivaxy.vscode-conventional-commits)
