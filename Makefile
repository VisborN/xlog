
PFILES=xlog.go rec_direct.go rec_syslog.go debugger.go errors.go

all: general additional

general:
	./tw.sh "xlog_test.go rec_direct_test.go logger_test.go $(PFILES)"

additional:
	./tw.sh "errors_test.go $(PFILES)"

concurrency:
	go test --run TestLoggerRaces --race
	go test --run TestLoggerDeadLocks --race
	go test --run TestRaceCondMsg --race
	go test --run TestRaceCondLoggerCalls --race
