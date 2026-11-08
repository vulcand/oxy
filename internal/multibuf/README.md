# What is this?

This is an altered copy of the `github.com/mailgun/multibuf@v0.2.0` module to fix bugs.

The original repository: https://github.com/mailgun/multibuf

## Why

The module is in slow maintenance and `oxy` is the main (and maybe the only one) consumer of this module.

To ease the maintenance, the module is now embedded as a third party package inside `oxy`. 
