# Changelog

## [v0.3.2] - 2020.05.07 [hotfix]

### Added

* global disable parameter
    It needs a way to fast and simple disable all logging code
    entries if it causes some errors or performance problems.
* `Logger` mutex to avoid race conditions _(yep, again xd)_

### Fixed

* __data races__
  * `LogMsg` in `WriteMsg()` _(pointer, same data)_
  * links on recorder in `xlog_test.go` _(partial fix)_
  * `recorder.listening` _(partial fix)_
    
## [v0.3.1] - ?

    * ...
    
## [v0.3.0] - ?

    * ...
