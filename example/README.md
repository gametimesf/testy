# Full example

This is a full example of how to use testy.
You would usually be testing an API hosted in another repository,
but for this example we are just writing a unit test for code in another package.

This example contains the following packages:
* `cmd` contains a binary to run a test server
* `fib` contains the code under test (a Fibonacci sequence generator)
* `tests` contains the tests

To run the tests as one would while writing tests, you can run:
```
go test -v ./tests
```

Which should provide output similar to the following:
```
=== RUN   TestFib
=== RUN   TestFib/100th_Fibonacci_number
=== RUN   TestFib/Fibonacci_number
=== RUN   TestFib/Fibonacci_number/0
=== RUN   TestFib/Fibonacci_number/1
=== RUN   TestFib/Fibonacci_number/2
=== RUN   TestFib/Fibonacci_number/3
=== RUN   TestFib/Fibonacci_number/4
=== RUN   TestFib/Fibonacci_number/5
=== RUN   TestFib/Fibonacci_number/6
=== RUN   TestFib/Fibonacci_number/7
=== RUN   TestFib/Fibonacci_number/8
=== RUN   TestFib/Fibonacci_number/9
=== RUN   TestFib/Fibonacci_number/10
--- PASS: TestFib (0.00s)
    --- PASS: TestFib/100th_Fibonacci_number (0.00s)
    --- PASS: TestFib/Fibonacci_number (0.00s)
        --- PASS: TestFib/Fibonacci_number/0 (0.00s)
        --- PASS: TestFib/Fibonacci_number/1 (0.00s)
        --- PASS: TestFib/Fibonacci_number/2 (0.00s)
        --- PASS: TestFib/Fibonacci_number/3 (0.00s)
        --- PASS: TestFib/Fibonacci_number/4 (0.00s)
        --- PASS: TestFib/Fibonacci_number/5 (0.00s)
        --- PASS: TestFib/Fibonacci_number/6 (0.00s)
        --- PASS: TestFib/Fibonacci_number/7 (0.00s)
        --- PASS: TestFib/Fibonacci_number/8 (0.00s)
        --- PASS: TestFib/Fibonacci_number/9 (0.00s)
        --- PASS: TestFib/Fibonacci_number/10 (0.00s)
PASS
ok      github.com/gametimesf/testy/example/tests       0.002s
```

You can use common `go test` flags:
```
# Ignore the test cache and force all tests to run.
go test -count=1 ./tests
# Run a specific test.
# Note that since all of the test cases are subtests of the top level TestFib bootstrap test,
# its name must be explicitly matched or the test framework will not know that the subtests exist.
go test -v -run 'TestFib/70th_Fibonacci_number' ./tests
go test -v -run 'TestFib/Fibonacci_number/5' ./tests
```

Test case names have all spaces replaced with underscores.

To run the tests via the API, you can run:
```
go run ./cmd
```

And then in another shell:
```
curl http://localhost:12345/tests/run | jq
```

Which should provide output similar to the following:
```json
{
  "Package": "",
  "Name": "Test Suite",
  "Msgs": null,
  "Result": "passed",
  "Started": "2023-08-10T15:52:24.264568354-07:00",
  "Dur": 0,
  "DurHuman": "0s",
  "Subtests": [
    {
      "Package": "github.com/gametimesf/testy/example/tests",
      "Name": "Package",
      "Msgs": null,
      "Result": "passed",
      "Started": "2023-08-10T15:52:24.26456986-07:00",
      "Dur": 0,
      "DurHuman": "0s",
      "Subtests": [
        {
          "Package": "github.com/gametimesf/testy/example/tests",
          "Name": "100th_Fibonacci_number",
          "Msgs": null,
          "Result": "passed",
          "Started": "2023-08-10T15:52:24.264573764-07:00",
          "Dur": 0,
          "DurHuman": "0s",
          "Subtests": null
        },
        {
          "Package": "github.com/gametimesf/testy/example/tests",
          "Name": "Fibonacci_number",
          "Msgs": null,
          "Result": "passed",
          "Started": "2023-08-10T15:52:24.264590332-07:00",
          "Dur": 0,
          "DurHuman": "0s",
          "Subtests": [
            {
              "Package": "github.com/gametimesf/testy/example/tests",
              "Name": "Fibonacci_number/0",
              "Msgs": null,
              "Result": "passed",
              "Started": "2023-08-10T15:52:24.26459676-07:00",
              "Dur": 0,
              "DurHuman": "0s",
              "Subtests": null
            },
            {
              "Package": "github.com/gametimesf/testy/example/tests",
              "Name": "Fibonacci_number/1",
              "Msgs": null,
              "Result": "passed",
              "Started": "2023-08-10T15:52:24.264608798-07:00",
              "Dur": 0,
              "DurHuman": "0s",
              "Subtests": null
            },
            {
              "Package": "github.com/gametimesf/testy/example/tests",
              "Name": "Fibonacci_number/2",
              "Msgs": null,
              "Result": "passed",
              "Started": "2023-08-10T15:52:24.26461284-07:00",
              "Dur": 0,
              "DurHuman": "0s",
              "Subtests": null
            },
            {
              "Package": "github.com/gametimesf/testy/example/tests",
              "Name": "Fibonacci_number/3",
              "Msgs": null,
              "Result": "passed",
              "Started": "2023-08-10T15:52:24.264616823-07:00",
              "Dur": 0,
              "DurHuman": "0s",
              "Subtests": null
            },
            {
              "Package": "github.com/gametimesf/testy/example/tests",
              "Name": "Fibonacci_number/4",
              "Msgs": null,
              "Result": "passed",
              "Started": "2023-08-10T15:52:24.264621337-07:00",
              "Dur": 0,
              "DurHuman": "0s",
              "Subtests": null
            },
            {
              "Package": "github.com/gametimesf/testy/example/tests",
              "Name": "Fibonacci_number/5",
              "Msgs": null,
              "Result": "passed",
              "Started": "2023-08-10T15:52:24.264630284-07:00",
              "Dur": 0,
              "DurHuman": "0s",
              "Subtests": null
            },
            {
              "Package": "github.com/gametimesf/testy/example/tests",
              "Name": "Fibonacci_number/6",
              "Msgs": null,
              "Result": "passed",
              "Started": "2023-08-10T15:52:24.264660257-07:00",
              "Dur": 0,
              "DurHuman": "0s",
              "Subtests": null
            },
            {
              "Package": "github.com/gametimesf/testy/example/tests",
              "Name": "Fibonacci_number/7",
              "Msgs": null,
              "Result": "passed",
              "Started": "2023-08-10T15:52:24.264670413-07:00",
              "Dur": 0,
              "DurHuman": "0s",
              "Subtests": null
            },
            {
              "Package": "github.com/gametimesf/testy/example/tests",
              "Name": "Fibonacci_number/8",
              "Msgs": null,
              "Result": "passed",
              "Started": "2023-08-10T15:52:24.264676395-07:00",
              "Dur": 0,
              "DurHuman": "0s",
              "Subtests": null
            },
            {
              "Package": "github.com/gametimesf/testy/example/tests",
              "Name": "Fibonacci_number/9",
              "Msgs": null,
              "Result": "passed",
              "Started": "2023-08-10T15:52:24.264696417-07:00",
              "Dur": 0,
              "DurHuman": "0s",
              "Subtests": null
            },
            {
              "Package": "github.com/gametimesf/testy/example/tests",
              "Name": "Fibonacci_number/10",
              "Msgs": null,
              "Result": "passed",
              "Started": "2023-08-10T15:52:24.264705707-07:00",
              "Dur": 0,
              "DurHuman": "0s",
              "Subtests": null
            }
          ]
        }
      ]
    }
  ]
}
```

After running the curl a few more times, open http://localhost:12345/tests/results/ to see the list of all test runs.
You can click on any of those to see the specifics for that run.
