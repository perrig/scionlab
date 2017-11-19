
# bwtester

The bandwidth testing application `bwtester` enables a variety of bandwidth tests on the SCION network. This document describes the design of the code and protocol. Instructions on the installation and usage are described in the main [README.md](https://github.com/perrig/scionlab/blob/master/README.md).

## Protocol design

The goal is to set up bandwidth test servers throughout the SCION network, which enable stress testing of the infrastructure.

To avoid server bottlenecks biasing the results, a server only allows a single client to perform a bandwidth test at a given point in time. Clients are served on a first-come-first-served basis. We limit the duration of each test to 10 seconds.

A bandwidth test is parametrized by the following parameters:


## bwtestclient



## bwtestserver

