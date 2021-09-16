# [Unreleased]

* Use CHANGELOG.md for release description (#306, @miry)
* Dependency updates in #294 introduced a breaking change in CLI argument parsing. Now [flags must be specified before arguments](https://github.com/urfave/cli/blob/master/docs/migrate-v1-to-v2.md#flags-before-args). Previously, arguments could be specified prior to flags.
  Update usage help text and documentation. (#308, @miry)
* Run e2e tests to validate the command line and basic features of server, client and application (#309, @miry)
* Add /v2 suffix to module import path (#311, @dnwe)
* Setup code linter (#314, @miry)
  * Max line length is 100 characters (#316, @miry)

# [2.1.5]

* Move to Go Modules from godeps (#253, @epk)
* Update the example in `client/README.md` (#251, @nothinux)
* Update TOC in `README.md` (4ca1eddddfcd0c50c8f6dfb97089bb68e6310fd9, @dwradcliffe)
* Add an example of `config.json` file to `README.md` (#260, @JesseEstum)
* Add Link to Elixir Client (#287, @Jcambass)
* Add Rust client link (#293, @itarato)
* Renovations: formatting code, update dependicies, make govet/staticcheck pass (#294, @dnwe)
* Remove `openssl` from `dev.yml` to use `dev` tool (#298, @pedro-stanaka)
* Update `go` versions in development (#299, @miry)
* Mention `MacPorts` in `README.md` (#290, @amake)
* Fix some typos in `README.md` and `CHANGELOG.md` (#222, @jwilk)
* Replace TravisCI with Github Actions to run tests (#303, @miry)
* Build and release binaries with `goreleaser`. Support `arm64` and BSD oses. (#301, @miry)
* Automate release with Github actions (#304, @miry)

# [2.1.4]

* Bug fix: Fix OOM in toxic. #232
* Documentation updates.
* CI and test updates.

# [2.1.3]

* Update `/version` endpoint to also return a charset of utf-8. #204
* Bug fix: Double http concatenation. #191
* Update cli examples to be more accurate. #187

# [2.1.2]

* go 1.8, make Sirupsen lower case, update godeps (issue #179)
* Handle SIGTERM to exit cleanly (issue #180)
* Address security issue by disallowing browsers from accessing API

# [2.1.1]

* Fix timeout toxic causing hang (issue #159)

# [2.1.0]

* Add -config server option to populate on startup #154
* Updated CLI for scriptability #133
* Add `/populate` endpoint to server #111
* Change error responses from `title` to `error`
* Allow hostname to be specified in CLI #129
* Add support for stateful toxics #127
* Add limit_data toxic

# [2.0.0]

* Add CLI (`toxiproxy-cli`) and rename server binary to `toxiproxy-server` #93
* Fix removing a timeout toxic causing API to hang #89
* API and client return toxics as array rather than a map of name to toxic #92
* Fix multiple latency toxics not accumulating #94
* Change default toxic name to `<type>_<stream>` #96
* Nest toxic attributes rather than having a flat structure #98
* 2.0 RFC: #54 and PR #62
    * Change toxic API endpoints to an Add/Update/Remove structure
    * Remove `enabled` field, and add `name` and `type` fields to toxics
    * Add global toxic fields to a wrapper struct
    * Chain toxics together dynamically instead of in a fixed length chain
    * Register toxics in `init()` functions instead of a hard-coded list
    * Clean up API error codes to make them more consistent
    * Move toxics to their own package to allow 3rd party toxics
* Remove stream direction from API urls #73
* Add `toxicity` field for toxics #75
* Refactor Go client to make usage easier with 2.0 #76
* Make `ChanReader` in the `stream` package interruptible #77
* Define proxy buffer sizes per-toxic (Fixes #72)
* Fix slicer toxic testing race condition #71

# [1.2.1]

* Fix proxy name conflicts leaking an open port #69

# [1.2.0]

* Add a Toxic and Toxics type for the Go client
* Add `Dockerfile`
* Fix latency toxic limiting bandwidth #67
* Add Slicer toxic

# [1.1.0]

* Remove /toxics endpoint in favour of /proxies
* Add bandwidth toxic

# [1.0.3]

* Rename Go library package to Toxiproxy from Client
* Fix latency toxic send to closed channel panic #46
* Fix latency toxic accumulating delay #47

# [1.0.2]

* Added Toxic support to Go client

# [1.0.1]

* Various improvements to the documentation
* Initial version of Go client
* Fix toxic disabling bug #42

# [1.0.0]

Initial public release.

[Unreleased]: https://github.com/Shopify/toxiproxy/compare/v2.1.5...HEAD
[2.1.5]: https://github.com/Shopify/toxiproxy/compare/v2.1.4...v2.1.5
[2.1.4]: https://github.com/Shopify/toxiproxy/compare/v2.1.3...v2.1.4
[2.1.3]: https://github.com/Shopify/toxiproxy/compare/v2.1.2...v2.1.3
[2.1.2]: https://github.com/Shopify/toxiproxy/compare/v2.1.1...v2.1.2
[2.1.1]: https://github.com/Shopify/toxiproxy/compare/v2.1.0...v2.1.1
[2.1.0]: https://github.com/Shopify/toxiproxy/compare/v2.0.0...v2.1.0
[2.0.0]: https://github.com/Shopify/toxiproxy/compare/v1.2.1...v2.0.0
[1.2.1]: https://github.com/Shopify/toxiproxy/compare/v1.2.0...v1.2.1
[1.2.0]: https://github.com/Shopify/toxiproxy/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/Shopify/toxiproxy/compare/v1.0.3...v1.1.0
[1.0.3]: https://github.com/Shopify/toxiproxy/compare/v1.0.2...v1.0.3
[1.0.2]: https://github.com/Shopify/toxiproxy/compare/v1.0.1...v1.0.2
[1.0.1]: https://github.com/Shopify/toxiproxy/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/Shopify/toxiproxy/releases/tag/v1.0.0
