#!/bin/bash -ex
# Copyright The OpenTelemetry Authors
# Modifications copyright New Relic, Inc.
#
# Modifications can be found at the following URL:
# https://github.com/newrelic/nrdot-collector-components/commits/main/.github/workflows/scripts/release-prepare-release.sh?since=2025-11-26
#
# SPDX-License-Identifier: Apache-2.0

PATTERN="^[0-9]+\.[0-9]+\.[0-9]+.*"
if ! [[ ${CURRENT_BETA} =~ $PATTERN ]]
then
    echo "CURRENT_BETA should follow a semver format and not be led by a v"
    exit 1
fi

if ! [[ ${CANDIDATE_BETA} =~ $PATTERN ]]
then
    echo "CANDIDATE_BETA should follow a semver format and not be led by a v"
    exit 1
fi

# Expand CURRENT_BETA to escape . character by using [.]
CURRENT_BETA_ESCAPED=${CURRENT_BETA//./[.]}

BRANCH="prepare-release-prs/${CANDIDATE_BETA}"
git checkout -b "${BRANCH}"

if [[ ${SYNC_UPSTREAM} == "true" ]]; then
    # Resolve the highest published OTel release tag at the candidate minor version so that
    # multimod sync operates against a known stable release rather than a floating main HEAD.
    # We query the Go module proxy directly rather than relying on local git tags, since a
    # shallow clone will not have historical tags for older minor versions.
    CANDIDATE_MINOR=$(echo "${CANDIDATE_BETA}" | cut -d. -f1-2)
    HIGHEST_OTEL_VERSION=$(cd cmd/nrdotcol && go list -m -versions go.opentelemetry.io/collector 2>/dev/null \
        | tr ' ' '\n' | grep "^v${CANDIDATE_MINOR}\." | grep -v -- '-' | sort -V | tail -1)
    if [[ -n "${HIGHEST_OTEL_VERSION}" ]]; then
        echo "Using OTel collector version: ${HIGHEST_OTEL_VERSION}"
        pushd ../opentelemetry-collector
        git fetch --depth=1 origin "refs/tags/${HIGHEST_OTEL_VERSION}:refs/tags/${HIGHEST_OTEL_VERSION}"
        git checkout "${HIGHEST_OTEL_VERSION}"
        popd
    else
        echo "Error: No published version found for v${CANDIDATE_MINOR}.x on the Go module proxy"
        exit 1
    fi

    # If the version is blank, multimod will use the version from upstream versions.yaml
    make update-otel OTEL_VERSION="" OTEL_STABLE_VERSION="" CONTRIB_VERSION=""

    # update-core-module-list updates based on upstream version.yaml
    make update-core-module-list
    git add internal/buildscripts/modules
    git commit -m "update core modules list" --allow-empty
else
    echo "Skipping upstream component updates"
fi

make chlog-update VERSION="v${CANDIDATE_BETA}"
git add --all
git commit -m "changelog update ${CANDIDATE_BETA}" || echo "no changelog changes to commit"

sed -i.bak "s/${CURRENT_BETA_ESCAPED}/${CANDIDATE_BETA}/g" versions.yaml
find . -name "*.bak" -type f -delete
git add versions.yaml
git commit -m "update version.yaml ${CANDIDATE_BETA}"

if [[ ${SYNC_UPSTREAM} == "true" ]]; then
    # Update all module versions
    sed -i.bak "s|v${CURRENT_BETA_ESCAPED}|v${CANDIDATE_BETA}|g" ./cmd/nrdotcol/builder-config.yaml
    sed -i.bak "s|v${CURRENT_BETA_ESCAPED}|v${CANDIDATE_BETA}|g" ./cmd/oteltestbedcol/builder-config.yaml
else
    # Only update nrdot module versions
    sed -i.bak "s|\(github\.com/newrelic/nrdot-collector-components/.* \)v${CURRENT_BETA_ESCAPED}|\1v${CANDIDATE_BETA}|g" ./cmd/nrdotcol/builder-config.yaml
    sed -i.bak "s|\(github\.com/newrelic/nrdot-collector-components/.* \)v${CURRENT_BETA_ESCAPED}|\1v${CANDIDATE_BETA}|g" ./cmd/oteltestbedcol/builder-config.yaml
fi
sed -i.bak "s|${CURRENT_BETA_ESCAPED}-dev|${CANDIDATE_BETA}-dev|g" ./cmd/nrdotcol/builder-config.yaml
sed -i.bak "s|${CURRENT_BETA_ESCAPED}-dev|${CANDIDATE_BETA}-dev|g" ./cmd/oteltestbedcol/builder-config.yaml

find . -name "*.bak" -type f -delete
make gennrdotcol
make genoteltestbedcol
git add .
git commit -m "builder config changes ${CANDIDATE_BETA}" || (echo "no builder config changes to commit")

make multimod-prerelease
git add .
git commit -m "make multimod-prerelease changes ${CANDIDATE_BETA}" || (echo "no multimod changes to commit")

pushd cmd/nrdotcol
go mod tidy
popd
make nrdotcol

git push --set-upstream origin "${BRANCH}"

gh pr create --head "$(git branch --show-current)" --title "[chore] Prepare release ${CANDIDATE_BETA}" --body "
The following commands were run to prepare this release:
- make chlog-update VERSION=v${CANDIDATE_BETA}
- sed -i.bak s/${CURRENT_BETA_ESCAPED}/${CANDIDATE_BETA}/g versions.yaml
- make multimod-prerelease
- make multimod-sync
"
