#!/usr/bin/env sh
set -e

UPGRADE_MATRIX_FILE=$1
IMAGES_LISTS_DIR=$2
RANCHERD_IMAGES_DIR=$3
IMAGES_LISTS_ARCHIVE_DIR=$4

WORKING_DIR=$(mktemp -d)

if [ ! -e "$UPGRADE_MATRIX_FILE" ]; then
  echo "Could not find $UPGRADE_MATRIX_FILE, skip it"
  exit 0
fi

# Add all the current version's images lists
mkdir -p "$IMAGES_LISTS_ARCHIVE_DIR"/current
find $IMAGES_LISTS_DIR -name "*.txt" -exec cat {} \; >> "$IMAGES_LISTS_ARCHIVE_DIR"/current/image_list_all.txt
find $RANCHERD_IMAGES_DIR -name "*.txt" -exec cat {} \; >> "$IMAGES_LISTS_ARCHIVE_DIR"/current/image_list_all.txt
cat "$IMAGES_LISTS_ARCHIVE_DIR"/current/image_list_all.txt | sort | uniq | tee "$IMAGES_LISTS_ARCHIVE_DIR"/current/image_list_all.txt

# Add all the previous versions' images lists
previous_versions=$(yq e ".versions[].name" "$UPGRADE_MATRIX_FILE" | xargs)
for prev_ver in $(echo "$previous_versions"); do
  ret=0

  echo "Fetching $prev_ver image lists..."

  mkdir "$WORKING_DIR"/"$prev_ver"

  curl -fL https://releases.rancher.com/harvester/"$prev_ver"/image-lists.tar.gz -o "$WORKING_DIR"/image-lists.tar.gz || ret=$?
  if [ "$ret" -ne 0 ]; then
    echo "Cannot download image list tarball for version $prev_ver, skip it"
    continue
  fi
  tar -zxvf "$WORKING_DIR"/image-lists.tar.gz -C "$WORKING_DIR"/"$prev_ver"/

  prev_image_list="$WORKING_DIR"/image_list_all.txt
  cat "$WORKING_DIR"/"$prev_ver"/image-lists/*.txt | sort | uniq > "$prev_image_list"

  mkdir -p "$IMAGES_LISTS_ARCHIVE_DIR"/"$prev_ver"
  cp -a "$prev_image_list" "$IMAGES_LISTS_ARCHIVE_DIR"/"$prev_ver"/

  rm -rf "$WORKING_DIR"/*
done
