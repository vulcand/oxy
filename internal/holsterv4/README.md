# What is this?

This is a vendored copy of 2 packages (`clock` and `collections`) from the
github.com/mailgun/holster@v4.2.5 module.

The `clock` package was completely copied over and the following modifications
were made:

* pkg/errors was replaced with the stdlib errors package / fmt.Errorf's %w;
* import names changed in blackbox test packages;
* a small race condition in the testing logic was fixed using the provided
  mutex.

The `collections` package only contains the priority_queue and ttlmap and
corresponding test files. The only changes made to those files were to adjust
the package names to use the vendored packages.

## Why

TL;DR: holster is a utility repo with many dependencies and even with graph
pruning using it in oxy can transitively impact oxy users in negative ways by
forcing version bumps (at the least).

Full details can be found here: https://github.com/vulcand/oxy/pull/223
