# A GoLang Interview Assignment

Fun exercise.  

This solution implements a host that listens on port 8080 for HTTP operations.

This was developed against GoLang 1.15.2 on MacOS Catalina 10.15.7.  Earlier Go
compilers (like sub 1.9 and earlier) might have issues building this.  

# Running

Pretty simple here:

    go run main.go

Or if you must have a binary:

    go build
    ./jmpc 

# Testing 

A unit test driver is implemented, to varying degrees of functionality.  In a professional or full time context 100% pass rate here would be a gate to a pull request acceptance:

    go test

# Design Notes

Hash results are persisted in an `sync.Map`, which uses RAM resources and will eventually exhaust at
high data and transacton volumes.  Off-board persistence could fix that, if requirements dictated so.

There are no persistence requirements for aging out older records nor for persisting to local block storage, thus none were implemented.   Since the record sizes are fixed between a uint64 and length
of Base 64 string, a simple offset based local file storage could be used.  In reaility these would
be passed off to some off-board persistence engine, relational, key-value, or otherwise.

Speaking of reality: hashing passwords really must include a salt, initialization vector, and
mechanisms to prevent from detecting when a user re-uses a password and when two users share the
same password.  Literature on how /etc/shadow works on modern unix systems can be used as a reference
for better ways to do this.  I realize we need consistent results for an interview test, that is
probably loaded into an automated validation tool :-).

The spec for the `average` statistic is somewhat ambiguous:

```The ​“average”​ key should have a value for the average time it has taken to process all of those requests in microseconds.```

It is unclear if this is meant to report average time for the GET request handler, or the hashing 
function for the request or the average of the sum of the two.  It is also unclear if the 5 second delay should be included in this metric.  This implementation elects to report the *sum* of the 
two: The time taken in the HTTP handler, summed with the time taken in the Hash calculation, not counting the 5 second delay.  Because hash calculations are delayed by a fixed amount of time there is a window where the statistics may be reported lower than actually needed.  Alternatively said: the
average time reports below the true value while there are outstanding hashes waiting for their 5 second delays. 

The spec is also ambiguous if calls to `/hash` that result in error should increment the processing
time metric.  This implementation *does* include that time, even if those results may not result in
a new hash being calculated. 