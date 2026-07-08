# arXiv Packaging

This repository includes a reproducible arXiv packaging workflow:

```sh
scripts/prepare_arxiv.sh
```

The script creates a temporary working directory, copies only the paper source and files required for compilation, flattens local LaTeX and figure paths, compiles the temporary copy, removes build artifacts, and writes:

```text
dist/arxiv/
dist/arxiv.tar
```

The manuscript source in `docs/paper/` is not modified by the script.

## What the Script Does

1. Copies `docs/paper/paper.tex` into a temporary directory as `main.tex`.
2. Extracts the embedded `thebibliography` block into `paper.bbl`.
3. Replaces the bibliography block in `main.tex` with `\input{paper.bbl}`.
4. Strips LaTeX comments from `.tex` files while preserving escaped percent signs (`\%`).
5. Flattens local `\input`, `\include`, and `\includegraphics` paths into the temporary root.
6. Removes `\graphicspath` because figure files are placed beside `main.tex`.
7. Appends the arXiv multiple-pass message after `\end{document}`.
8. Runs multiple `pdflatex` passes inside the temporary directory and fails if compilation fails.
9. Removes auxiliary files, PDFs, `.bib` files, Git/editor metadata, and any nested directories.
10. Copies the clean flat source tree to `dist/arxiv/`.
11. Creates `dist/arxiv.tar` containing only the flat arXiv source files.

## Regenerating the Package

Run this from the repository root whenever the paper changes:

```sh
scripts/prepare_arxiv.sh
```

The command is safe to rerun. It recreates `dist/arxiv/` and `dist/arxiv.tar` from scratch each time.

## Uploading to arXiv

1. Run `scripts/prepare_arxiv.sh`.
2. Upload `dist/arxiv.tar` through the arXiv submission form.
3. Select `main.tex` as the primary TeX file if arXiv asks.
4. Confirm that arXiv compiles the source successfully.
5. Review the generated PDF in arXiv before final submission.

## Metadata Checklist

Title: Implementing and Evaluating Skeen's Atomic Multicast Protocol and an ACK-Gated Extension for Atomic Global Order

Abstract: Atomic multicast is a central communication abstraction for partitioned state machine replication and distributed key-value stores because it orders operations only at the partitions they access while preserving consistency across overlapping destination sets. Skeen's decentralized atomic multicast protocol provides a compact timestamp-based ordering mechanism, but later work shows that the original protocol admits unsafe executions when overlapping multicasts expose real-time dependencies before all destinations have converged on the same delivery barrier. This paper implements and evaluates two protocols in a distributed key-value store: original Skeen and the ACK-gated extension proposed by Pacheco et al. The system supports generic N-partition routing, configurable membership, arbitrary multicast destination sets, in-memory and HTTP transports, Docker deployment, and structured logging. Its main engineering contribution is a deterministic validation framework that controls message delivery using scheduled transport queues, predicate-based release rules, delivery trace recording, and cycle detection. The framework reproduces a representative unsafe execution for original Skeen without sleeps or timing races and shows that the ACK-gated variant prevents premature delivery for that same scheduled counterexample. This is counterexample reproduction and regression validation, not exhaustive safety verification. We also benchmark both protocols for cluster sizes 2, 3, and 5 and destination sets up to the full cluster under injected delays of 0, 1, 5, and 10 ms. The results quantify the cost of the acknowledgement barrier: ACK gating adds measurable CPU and allocation overhead in the zero-delay case, and the extra control-message dependency becomes visible under artificial phase-counting latency.

Author list: Agatha Schneider

Suggested arXiv category: cs.DC

License selection reminder: choose the arXiv license intentionally during submission, based on the intended reuse rights for the paper.
