#!/usr/bin/env python3
import csv
import os
import statistics
import sys
from collections import defaultdict

import matplotlib.pyplot as plt


def load_rows(path):
    with open(path, "r", encoding="utf-8") as input_file:
        rows = []
        for row in csv.DictReader(input_file):
            row["n"] = int(row["n"] or 0)
            row["dst"] = int(row["dst"] or 0)
            row["delay_ms"] = int(row["delay_ms"] or 0)
            row["ack_delay_ms"] = int(row["ack_delay_ms"] or 0)
            row["ns_per_op"] = float(row["ns_per_op"] or 0)
            row["ms_per_op"] = row["ns_per_op"] / 1_000_000
            rows.append(row)
        return rows


def averaged_series(rows, x_key, series_key):
    grouped = defaultdict(list)
    for row in rows:
        grouped[(row[series_key], row[x_key])].append(row["ms_per_op"])

    series = defaultdict(list)
    for (series_name, x_value), values in grouped.items():
        series[series_name].append((x_value, statistics.mean(values)))

    for series_name in series:
        series[series_name].sort(key=lambda item: item[0])
    return series


def plot_series(series, xlabel, ylabel, title, output_path):
    plt.figure(figsize=(8, 5))
    for series_name, points in sorted(series.items()):
        xs = [point[0] for point in points]
        ys = [point[1] for point in points]
        plt.plot(xs, ys, marker="o", label=series_name)
    plt.xlabel(xlabel)
    plt.ylabel(ylabel)
    plt.title(title)
    plt.grid(True, alpha=0.3)
    plt.legend()
    plt.tight_layout()
    plt.savefig(output_path, dpi=160)
    plt.close()


def latency_vs_delay(rows, output_dir):
    filtered = [
        row
        for row in rows
        if row["benchmark"] == "BenchmarkSkeenArtificialLatency"
        and row["n"] == 3
        and row["dst"] == 3
        and row["ack_delay_ms"] == 0
    ]
    plot_series(
        averaged_series(filtered, "delay_ms", "mode"),
        "delay (ms)",
        "latency (ms/op)",
        "N=3 dst=3 latency sensitivity",
        os.path.join(output_dir, "latency_vs_delay.png"),
    )


def overhead_by_dst(rows, output_dir):
    filtered = [
        row
        for row in rows
        if row["benchmark"] in {"BenchmarkSkeenArtificialLatency", "BenchmarkSkeenDestinationOverhead"}
        and row["n"] == 3
        and row["delay_ms"] == 0
        and row["ack_delay_ms"] == 0
    ]
    plot_series(
        averaged_series(filtered, "dst", "mode"),
        "destination count",
        "latency (ms/op)",
        "N=3 destination overhead at zero artificial delay",
        os.path.join(output_dir, "overhead_by_dst.png"),
    )


def ack_delay_overhead(rows, output_dir):
    filtered = [
        row
        for row in rows
        if row["benchmark"] == "BenchmarkSkeenAckLatency"
        and row["n"] == 3
        and row["dst"] == 3
        and row["mode"] == "strengthened"
    ]
    plot_series(
        averaged_series(filtered, "ack_delay_ms", "mode"),
        "ACK delay (ms)",
        "latency (ms/op)",
        "N=3 strengthened ACK delay overhead",
        os.path.join(output_dir, "ack_delay_overhead.png"),
    )


def main():
    if len(sys.argv) != 2:
        print("usage: plot_benchmarks.py <benchmark-csv>", file=sys.stderr)
        return 2

    rows = load_rows(sys.argv[1])
    output_dir = os.path.join(os.path.dirname(sys.argv[1]) or ".", "plots")
    os.makedirs(output_dir, exist_ok=True)

    latency_vs_delay(rows, output_dir)
    overhead_by_dst(rows, output_dir)
    ack_delay_overhead(rows, output_dir)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
