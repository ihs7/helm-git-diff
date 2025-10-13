#!/bin/bash
set -e

VERSION=$(grep '^version:' plugin.yaml | awk '{print $2}' | tr -d '"')

if [ -z "$VERSION" ]; then
  echo "Error: Could not extract version from plugin.yaml"
  exit 1
fi

TAG="v${VERSION}"

if git rev-parse "$TAG" >/dev/null 2>&1; then
  echo "Error: Tag $TAG already exists"
  exit 1
fi

echo "Creating release for version $VERSION"
echo "Tag: $TAG"
echo

read -p "Continue? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
  echo "Aborted"
  exit 1
fi

git tag -a "$TAG" -m "Release $TAG"
echo "Tag $TAG created"
echo
echo "Push with: git push origin $TAG"
