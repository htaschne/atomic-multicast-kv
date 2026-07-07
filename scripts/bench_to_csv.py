#!/usr/bin/env python3
import csv
import re
import sys


BENCH_RE = re.compile(
    r"^(Benchmark\S+)\s+\d+\s+([\d.]+)\s+ns/op(?:\s+([\d.]+)\s+B/op)?(?:\s+([\d.]+)\s+allocs/op)?"
)


def duration_ms(value):
    if value == "":
        return 0
    if value.endswith("ms"):
        return int(value[:-2])
    if value.endswith("s"):
        seconds = float(value[:-1])
        return int(seconds * 1000)
    if value.endswith("us") or value.endswith("µs"):
        return int(float(value[:-2]) / 1000)
    if value.endswith("ns"):
        return int(float(value[:-2]) / 1_000_000)
    return int(value)


def parse_benchmark_name(raw_name):
    name = re.sub(r"-\d+$", "", raw_name)
    parts = name.split("/")
    benchmark = parts[0]
    fields = {
        "benchmark": benchmark,
        "n": "",
        "mode": "",
        "dst": "",
        "delay_ms": "0",
        "ack_delay_ms": "0",
    }

    for part in parts[1:]:
        if "=" not in part:
            continue
        key, value = part.split("=", 1)
        if key == "N":
            fields["n"] = value
        elif key == "mode":
            fields["mode"] = value
        elif key == "dst":
            fields["dst"] = value
        elif key == "delay":
            fields["delay_ms"] = str(duration_ms(value))
        elif key == "ackDelay":
            fields["ack_delay_ms"] = str(duration_ms(value))

    return fields


def parse_lines(lines):
    for line in lines:
        match = BENCH_RE.match(line.strip())
        if not match:
            continue

        fields = parse_benchmark_name(match.group(1))
        if fields["benchmark"] not in {
            "BenchmarkSkeenDestinationOverhead",
            "BenchmarkSkeenArtificialLatency",
            "BenchmarkSkeenAckLatency",
        }:
            continue

        fields["ns_per_op"] = match.group(2)
        fields["b_per_op"] = match.group(3) or ""
        fields["allocs_per_op"] = match.group(4) or ""
        yield fields


def main():
    if len(sys.argv) != 2:
        print("usage: bench_to_csv.py <go-benchmark-output>", file=sys.stderr)
        return 2

    columns = [
        "benchmark",
        "n",
        "mode",
        "dst",
        "delay_ms",
        "ack_delay_ms",
        "ns_per_op",
        "b_per_op",
        "allocs_per_op",
    ]
    with open(sys.argv[1], "r", encoding="utf-8") as input_file:
        writer = csv.DictWriter(sys.stdout, fieldnames=columns)
        writer.writeheader()
        for row in parse_lines(input_file):
            writer.writerow(row)

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
