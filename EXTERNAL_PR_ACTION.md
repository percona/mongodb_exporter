# External PR Trigger GitHub Action

This GitHub Action automatically triggers actions in a target repository when a pull request is created by users who are not in an allowed list.

## Overview

When a pull request is opened, synchronized, or reopened from a branch that does NOT have a `PMM-` prefix, this action will:
1. Create a new branch in a specified target repository
2. Modify a `ci.yml` file in that repository with PR information
3. Create a pull request in the target repository

## Setup Instructions

### 1. Understanding the Branch Prefix Check

The workflow file `.github/workflows/external-pr-trigger.yml` checks the source branch name:

```yaml
if: |
  !startsWith(github.event.pull_request.head.ref, 'PMM-')
```

This means:
- PRs from branches WITH the `PMM-` prefix will NOT trigger this action
- PRs from branches WITHOUT the `PMM-` prefix WILL trigger this action

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

The action creates/updates a `ci.yml` file in the target repository. You can customize the content by modifying this section in the workflow:

```yaml
# Modify ci.yml file with PR information
cat > ci.yml << EOF
# Auto-generated from external PR
external_pr:
  deps:
    - name: mongodb_exporter
      url: https://github.com/percona/mongodb_exporter
      branch: branch-name
EOF
```

## How It Works

1. **Trigger**: The action runs on every pull request event (opened, synchronized, reopened)

3. **Branch Creation**: For external users, it:
   - Clones the target repository
   - Creates a new branch named `external-pr-{original-branch-name}`
   - Updates the `ci.yml` file with PR metadata

4. **Pull Request**: Creates a pull request in the target repository with:
   - Title: "External PR: {original PR title}"
   - Body: Contains a link to the original PR and its description

## Security Considerations

1. **Token Security**: The PAT is stored as a secret and never exposed in logs
2. **Limited Scope**: The action only modifies the specified `ci.yml` file
3. **Branch Filtering**: Only PRs from branches without the `PMM-` prefix trigger the action

## Troubleshooting

### Action Not Triggering
- Verify the branch does NOT have a `PMM-` prefix
- Check that the workflow file is in `.github/workflows/` directory
- Ensure the workflow has the correct event triggers

### Permission Errors
- Verify the PAT has the correct scopes
- Check that the token hasn't expired
- Ensure the target repository allows the token's access

### Branch/PR Creation Fails
- Check that the target repository exists
- Verify the `TARGET_REPO_OWNER` and `TARGET_REPO_NAME` variables are correct
- Ensure there isn't already a branch with the same name

## Example Scenarios

### Scenario 1: PR from non-PMM branch (Action triggers)
1. User creates PR #123 from branch `fix-bug` (no PMM- prefix)
2. This action triggers and:
   - Creates branch `external-pr-fix-bug` in the target repository
   - Updates `ci.yml` with PR #123's information
   - Creates a PR in the target repository titled "External PR: Fix bug"
3. The target repository can then run its own CI/CD processes based on the `ci.yml` content

### Scenario 2: PR from PMM branch (Action does NOT trigger)
1. User creates PR #124 from branch `PMM-1234-fix-issue`
2. This action does NOT trigger because the branch has the `PMM-` prefix
3. The PR proceeds with normal repository workflows
