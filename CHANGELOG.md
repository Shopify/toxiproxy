# 2.0.0 (Unreleased)

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
* API and client return toxics as array rather than a map of name to toxic #92
* Nest toxic attributes rather than having a flat structure #98

# 1.2.1

* Fix proxy name conflicts leaking an open port #69

# 1.2.0

* Add a Toxic and Toxics type for the Go client
* Add `Dockerfile`
* Fix latency toxic limiting bandwidth #67
* Add Slicer toxic

# 1.1.0

* Remove /toxics endpoint in favour of /proxies
* Add bandwidth toxic

# 1.0.3

* Rename Go library package to Toxiproxy from Client
* Fix latency toxic send to closed channel panic #46
* Fix latency toxic accumulating delay #47

# 1.0.2

* Added Toxic support to Go client

# 1.0.1

* Various improvements to the documentation
* Initial version of Go client
* Fix toxic disabling bug #42

# 1.0.0

Initial public release.
