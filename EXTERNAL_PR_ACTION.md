# External PR Trigger GitHub Action

This GitHub Action automatically triggers actions in a target repository when a pull request is created or when commits are pushed to branches that don't have a `PMM-` prefix.

## Overview

When a pull request is opened, synchronized, or reopened, OR when commits are pushed to a branch that does NOT have a `PMM-` prefix, this action will:
1. Create a new branch in a specified target repository
2. Modify a `ci.yml` file in that repository with PR/push information
3. Create a pull request in the target repository (or update if one already exists)

## Trigger Events

The action triggers on:
- **Pull Request events**: opened, synchronized, reopened
- **Push events**: to any branch except `main` and `master`

In both cases, the action only runs if the branch does NOT have a `PMM-` prefix.

## Setup Instructions

### 1. Understanding the Branch Prefix Check

The workflow checks the source branch name:

```yaml
if: |
  (github.event_name == 'pull_request' && !startsWith(github.event.pull_request.head.ref, 'PMM-')) ||
  (github.event_name == 'push' && !startsWith(github.ref_name, 'PMM-'))
```

This means:
- Branches WITH the `PMM-` prefix will NOT trigger this action
- Branches WITHOUT the `PMM-` prefix WILL trigger this action

### 2. Create a Personal Access Token

You need a Personal Access Token (PAT) with permissions to create branches and pull requests in the target repository:

1. Go to GitHub Settings → Developer settings → Personal access tokens
2. Generate a new token with the following scopes:
   - `repo` (full control of private repositories)
   - `workflow` (if the target repo has GitHub Actions)
3. Copy the generated token

### 3. Configure Repository Secrets

In your repository settings, go to Secrets and variables → Actions, and add:

- **SECRET**: `TARGET_REPO_TOKEN` - The Personal Access Token you created

### 4. Configure Repository Variables

In your repository settings, go to Secrets and variables → Actions → Variables tab, and add:

- **VARIABLE**: `TARGET_REPO_OWNER` - The owner/organization of the target repository
- **VARIABLE**: `TARGET_REPO_NAME` - The name of the target repository

Example:
- `TARGET_REPO_OWNER`: `myorg`
- `TARGET_REPO_NAME`: `ci-configs`

### 5. Customize the ci.yml Content (Optional)

The action creates/updates a `ci.yml` file in the target repository. The content includes:

```yaml
# Auto-generated from external PR
external_pr:
  source_repo: <repository>
  pr_number: <PR number or N/A for pushes>
  pr_author: <author>
  pr_branch: <branch name>
  pr_title: <PR title or push description>
  pr_url: <PR URL or branch URL>
  triggered_at: <timestamp>
  event_type: <pull_request or push>
  commit_sha: <commit SHA>
```

## How It Works

### For Pull Requests

1. **Trigger**: PR is opened, synchronized, or reopened
2. **Branch Check**: Verifies the PR branch doesn't have `PMM-` prefix
3. **Target Branch**: Creates/updates branch `external-pr-{source-branch-name}`
4. **PR Creation**: Creates a PR titled "External PR: {original PR title}"

### For Push Events

1. **Trigger**: Commits are pushed to a branch (not main/master)
2. **Branch Check**: Verifies the branch doesn't have `PMM-` prefix
3. **Target Branch**: Creates/updates branch `external-pr-{source-branch-name}`
4. **PR Handling**:
   - If no PR exists: Creates one titled "External Push: {branch name}"
   - If PR exists: Updates the branch and adds a comment to the existing PR

## Key Features

### Duplicate PR Prevention
The action checks if a PR already exists for the branch in the target repository:
- If no PR exists: Creates a new one
- If PR exists: Updates the branch and comments on the existing PR

### Event-Specific Information
The `ci.yml` file includes different metadata based on the triggering event:
- **Pull Request**: Includes PR number, title, and description
- **Push**: Includes branch name, pusher, and commit SHA

## Security Considerations

1. **Token Security**: The PAT is stored as a secret and never exposed in logs
2. **Limited Scope**: The action only modifies the specified `ci.yml` file
3. **Branch Filtering**: Only branches without the `PMM-` prefix trigger the action
4. **Protected Branches**: Pushes to `main` and `master` are excluded

## Troubleshooting

### Action Not Triggering
- Verify the branch does NOT have a `PMM-` prefix
- Check that the workflow file is in `.github/workflows/` directory
- Ensure the event (PR or push) matches the configured triggers
- For pushes, verify the branch is not `main` or `master`

### Permission Errors
- Verify the PAT has the correct scopes
- Check that the token hasn't expired
- Ensure the target repository allows the token's access

### Branch/PR Creation Fails
- Check that the target repository exists
- Verify the `TARGET_REPO_OWNER` and `TARGET_REPO_NAME` variables are correct
- Check for existing branches with conflicting names

## Example Scenarios

### Scenario 1: New PR from non-PMM branch
1. User creates PR #123 from branch `fix-bug`
2. Action creates branch `external-pr-fix-bug` in target repo
3. Creates PR titled "External PR: Fix bug title"

### Scenario 2: Push to existing branch with PR
1. User pushes to branch `feature-xyz` (PR already exists in target)
2. Action updates branch `external-pr-feature-xyz`
3. Adds comment to existing PR about the update

### Scenario 3: Push to new branch without PR
1. User pushes to new branch `hotfix-123`
2. Action creates branch `external-pr-hotfix-123` in target repo
3. Creates PR titled "External Push: hotfix-123"

### Scenario 4: PMM branch (Action does NOT trigger)
1. User creates PR or pushes to branch `PMM-1234-fix-issue`
2. Action does NOT trigger due to `PMM-` prefix