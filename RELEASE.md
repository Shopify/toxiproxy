# Releasing

- [Releasing](#releasing)
  - [Before You Begin](#before-you-begin)
  - [Local Release Preparation](#local-release-preparation)
    - [Checkout latest code](#checkout-latest-code)
    - [Update the CHANGELOG.md](#update-the-changelogmd)
    - [Create Release Commit and Tag](#create-release-commit-and-tag)
    - [Run Pre-Release Tests](#run-pre-release-tests)
  - [Push Release Tag](#push-release-tag)
  - [Verify Github Release](#verify-github-release)
  - [Update Homebrew versions](#update-homebrew-versions)

## Before You Begin

Ensure your local workstation is configured to be able to [Sign commits](https://docs.github.com/en/authentication/managing-commit-signature-verification/signing-commits)

## Local Release Preparation

### Checkout latest code

```shell
git checkout main
git pull origin main
```

### Update the [CHANGELOG.md](CHANGELOG.md)

- Add a new version header at the top of the document, just after `# [Unreleased]`
- Update links at bottom of changelog

### Create Release Commit and Tag

```shell
export RELEASE_VERSION=2.x.y
git commit -a -S -m "Release $RELEASE_VERSION"
git tag -s "v$RELEASE_VERSION" # When prompted for a commit message, enter the 'release notes' style message, just like on the releases page
```

### Run Pre-Release Tests

```shell
make test-release
```

- Push to Main Branch
```shell
git push origin main --follow-tags
```

## Push Release Tag

- On your local machine again, push your tag to github

```shell
git push origin "v$RELEASE_VERSION"
```

## Verify Github Release

- Github Actions should kick off a build and release after the tag is pushed.
- Verify that a [Release gets created in Github](https://github.com/Shopify/toxiproxy/releases) and verify that the release notes look correct
- Github Actions should also attatch the built binaries to the release (it might take a few mins)

## Update Homebrew versions

- Update [homebrew-shopify toxiproxy.rb](https://github.com/Shopify/homebrew-shopify/blob/master/toxiproxy.rb#L9) manifest
  1. Update `app_version` string to your released version
  2. Update hashes for all platforms (find the hashes in the checksums.txt from your release notes)

- Do a manual check of installing toxiproxy via brew
  1. While in the homebrew-shopify directory...
  ```shell
  brew install ./toxiproxy.rb --debug
  ```
  Note: it's normal to get some errors when homebrew attempts to load the file as a Cask instead of a formula, just make sure that it still gets installed.
- PR the version update change and merge
