#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC_DIR="$ROOT_DIR/docs/paper"
SRC_TEX="$SRC_DIR/paper.tex"
DIST_DIR="$ROOT_DIR/dist/arxiv"
TAR_FILE="$ROOT_DIR/dist/arxiv.tar"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/atomic-multicast-arxiv.XXXXXX")"

cleanup() {
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

require_file() {
  [ -f "$1" ] || fail "required file not found: $1"
}

require_file "$SRC_TEX"
command -v pdflatex >/dev/null 2>&1 || fail "pdflatex is required"

mkdir -p "$ROOT_DIR/dist"
rm -rf "$DIST_DIR" "$TAR_FILE"
mkdir -p "$WORK_DIR"

strip_tex_comments() {
  local file="$1"
  perl -i -pe '
    chomp;
    my $out = "";
    my $slashes = 0;
    for my $c (split //) {
      if ($c eq "%" && $slashes % 2 == 0) {
        last;
      }
      $out .= $c;
      if ($c eq "\\") {
        $slashes++;
      } else {
        $slashes = 0;
      }
    }
    $_ = $out . "\n";
  ' "$file"
}

has_unescaped_percent() {
  local file="$1"
  perl -ne '
    chomp;
    my $slashes = 0;
    for my $c (split //) {
      if ($c eq "%" && $slashes % 2 == 0) {
        exit 1;
      }
      if ($c eq "\\") {
        $slashes++;
      } else {
        $slashes = 0;
      }
    }
  ' "$file"
}

copy_unique() {
  local src="$1"
  local dst="$2"
  require_file "$src"
  if [ -e "$dst" ] && ! cmp -s "$src" "$dst"; then
    fail "flattening collision for $(basename "$dst")"
  fi
  cp "$src" "$dst"
}

resolve_existing_file() {
  local rel="$1"
  local kind="$2"
  local candidate

  rel="${rel#./}"
  case "$kind" in
    tex)
      for candidate in "$SRC_DIR/$rel" "$SRC_DIR/$rel.tex"; do
        [ -f "$candidate" ] && { printf '%s\n' "$candidate"; return 0; }
      done
      ;;
    graphic)
      if [ -f "$SRC_DIR/$rel" ]; then
        printf '%s\n' "$SRC_DIR/$rel"
        return 0
      fi
      for candidate in "$SRC_DIR/$rel".pdf "$SRC_DIR/$rel".png "$SRC_DIR/$rel".jpg "$SRC_DIR/$rel".jpeg "$SRC_DIR/$rel".eps; do
        [ -f "$candidate" ] && { printf '%s\n' "$candidate"; return 0; }
      done
      ;;
  esac

  return 1
}

rewrite_paths_to_basenames() {
  local file="$1"
  perl -MFile::Basename -0pi -e '
    s/(\\includegraphics(?:\[[^\]]*\])?\{)([^}]+)(\})/$1 . basename($2) . $3/ge;
    s/(\\(?:input|include)\{)([^}]+)(\})/$1 . basename($2) . $3/ge;
  ' "$file"
}

extract_references() {
  local file="$1"
  perl -ne '
    while (/\\includegraphics(?:\[[^\]]*\])?\{([^}]+)\}/g) {
      print "graphic\t$1\n";
    }
    while (/\\(?:input|include)\{([^}]+)\}/g) {
      print "tex\t$1\n";
    }
  ' "$file"
}

prepare_tex_file() {
  local file="$1"
  strip_tex_comments "$file"
  perl -0pi -e 's/^\s*\\graphicspath\{[^\n]*\}\s*\n//mg' "$file"
}

awk -v bbl="$WORK_DIR/paper.bbl" '
  index($0, "\\begin{thebibliography}") {
    in_bib = 1
    print "\\input{paper.bbl}"
    print > bbl
    next
  }
  in_bib {
    print > bbl
    if (index($0, "\\end{thebibliography}")) {
      in_bib = 0
    }
    next
  }
  { print }
' "$SRC_TEX" > "$WORK_DIR/main.tex"

[ -s "$WORK_DIR/paper.bbl" ] || fail "paper.tex does not contain an embedded thebibliography block"
prepare_tex_file "$WORK_DIR/main.tex"

perl -0pi -e 's/\\end\{document\}\s*\z/\\end{document}\n\\typeout{get arXiv to do 4 passes: Label(s) may have changed. Rerun}\n/s' "$WORK_DIR/main.tex"
grep -Fq '\typeout{get arXiv to do 4 passes: Label(s) may have changed. Rerun}' "$WORK_DIR/main.tex" \
  || fail "failed to append arXiv rerun message"

queue=("main.tex")
processed=""

while [ "${#queue[@]}" -gt 0 ]; do
  current="${queue[0]}"
  queue=("${queue[@]:1}")

  case " $processed " in
    *" $current "*) continue ;;
  esac
  processed="$processed $current"

  current_path="$WORK_DIR/$current"
  require_file "$current_path"

  while IFS=$'\t' read -r kind rel; do
    [ -n "${kind:-}" ] || continue
    [ "$rel" = "paper.bbl" ] && continue

    if src="$(resolve_existing_file "$rel" "$kind")"; then
      base="$(basename "$src")"
      copy_unique "$src" "$WORK_DIR/$base"
      if [ "$kind" = "tex" ]; then
        prepare_tex_file "$WORK_DIR/$base"
        queue+=("$base")
      fi
    else
      fail "referenced local $kind file not found: $rel"
    fi
  done < <(extract_references "$current_path")

  rewrite_paths_to_basenames "$current_path"
done

(
  cd "$WORK_DIR"
  for pass in 1 2 3; do
    if ! pdflatex -interaction=nonstopmode -halt-on-error main.tex >"pdflatex-pass-${pass}.log" 2>&1; then
      cat "pdflatex-pass-${pass}.log" >&2
      exit 1
    fi
  done
)

find "$WORK_DIR" -type f \( \
  -name '*.aux' -o -name '*.out' -o -name '*.toc' -o -name '*.log' \
  -o -name '*.synctex.gz' -o -name '*.pdf' -o -name '*.fls' \
  -o -name '*.fdb_latexmk' -o -name '*.blg' -o -name '*.bcf' \
  -o -name '*.run.xml' -o -name '*.lof' -o -name '*.lot' \
  -o -name '*.nav' -o -name '*.snm' -o -name '*.vrb' \
  -o -name '*.xdv' -o -name '*.dvi' -o -name '*.ps' \
\) -delete
find "$WORK_DIR" -type f -name '*.bib' -delete
find "$WORK_DIR" -type d \( -name '.git' -o -name '.github' -o -name '.vscode' -o -name '.idea' \) -prune -exec rm -rf {} +

mkdir -p "$DIST_DIR"
cp -R "$WORK_DIR"/. "$DIST_DIR"/

(
  cd "$DIST_DIR"
  find . -mindepth 1 -type d -print -quit | grep -q . && fail "dist/arxiv is not flat"
  find . \( -name '.git' -o -name '.github' -o -name '.vscode' -o -name '.idea' \) -print -quit | grep -q . \
    && fail "Git or editor metadata remains in dist/arxiv"
  find . -type f \( \
    -name '*.aux' -o -name '*.out' -o -name '*.toc' -o -name '*.log' \
    -o -name '*.synctex.gz' -o -name '*.pdf' -o -name '*.fls' \
    -o -name '*.fdb_latexmk' -o -name '*.blg' -o -name '*.bcf' \
    -o -name '*.run.xml' -o -name '*.lof' -o -name '*.lot' \
    -o -name '*.nav' -o -name '*.snm' -o -name '*.vrb' \
    -o -name '*.xdv' -o -name '*.dvi' -o -name '*.ps' \
  \) -print -quit | grep -q . && fail "auxiliary files remain in dist/arxiv"
  find . -type f -name '*.bib' -print -quit | grep -q . && fail ".bib files remain in dist/arxiv"
  [ -f paper.bbl ] || fail "paper.bbl missing from dist/arxiv"
  [ -f main.tex ] || fail "main.tex missing from dist/arxiv"
  grep -Fq '\typeout{get arXiv to do 4 passes: Label(s) may have changed. Rerun}' main.tex \
    || fail "arXiv rerun message missing"
  while IFS= read -r tex_file; do
    has_unescaped_percent "$tex_file" || fail "unstripped LaTeX comment remains in $tex_file"
  done < <(find . -maxdepth 1 -type f -name '*.tex' | sort)
)

tar_files=()
while IFS= read -r file; do
  tar_files+=("$file")
done < <(cd "$DIST_DIR" && find . -maxdepth 1 -type f -print | sed 's#^\./##' | sort)

(
  cd "$DIST_DIR"
  tar -cf "$TAR_FILE" "${tar_files[@]}"
)

printf '✓ builds successfully\n'
printf '✓ no auxiliary files\n'
printf '✓ no Git metadata\n'
printf '✓ bibliography precompiled (.bbl)\n'
printf '✓ .bib removed\n'
printf '✓ comments stripped\n'
printf '✓ flattened directory\n'
printf '✓ arXiv rerun message present\n'
printf '\nCreated %s\n' "$DIST_DIR"
printf 'Created %s\n' "$TAR_FILE"
