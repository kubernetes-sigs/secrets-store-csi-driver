#!/usr/bin/env bash

# Copyright 2020 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

TASK=$1

pushd "$(dirname "$0")"

LDFLAGS="-X sigs.k8s.io/secrets-store-csi-driver/pkg/version.BuildVersion=${IMAGE_VERSION} \
 -X sigs.k8s.io/secrets-store-csi-driver/pkg/version.Vcs=${BUILD_COMMIT} \
 -X sigs.k8s.io/secrets-store-csi-driver/pkg/version.BuildTime=${BUILD_TIMESTAMP} -extldflags '-static'"

QEMUVERSION=5.2.0-2

# Returns list of all supported architectures from the BASEIMAGE file
listOsArchs() {
  cut -d "=" -f 1 BASEIMAGE
}

splitOsArch() {
  os_arch=$1

  if [[ $os_arch =~ .*/.*/.* ]]; then
    # for Windows, we have to support both LTS and SAC channels, so we're building multiple Windows images.
    # the format for this case is: OS/ARCH/OS_VERSION.
    os_name=$(echo "$os_arch" | cut -d "/" -f 1)
    arch=$(echo "$os_arch" | cut -d "/" -f 2)
    os_version=$(echo "$os_arch" | cut -d "/" -f 3)
    suffix="$os_name-$arch-$os_version"
  elif [[ $os_arch =~ .*/.* ]]; then
    os_name=$(echo "$os_arch" | cut -d "/" -f 1)
    arch=$(echo "$os_arch" | cut -d "/" -f 2)
    suffix="$os_name-$arch"
  else
    echo "The BASEIMAGE file is not properly formatted. Expected entries to start with 'os/arch', found '${os_arch}' instead."
    exit 1
  fi
}

# Returns baseimage need to used in Dockerfile for any given architecture
getBaseImage() {
  os_arch=$1
  file=${2:-BASEIMAGE}
  grep "${os_arch}=" "${file}" | cut -d= -f2
}

docker_version_check() {
  # The reason for this version check is even though "docker manifest" command is available in 18.03, it does
  # not work properly in that version. So we insist on 18.06.0 or higher.
  # docker buildx has been introduced in 19.03, so we need to make sure we have it.
  docker_version=$(docker version --format '{{.Client.Version}}' | cut -d"-" -f1)
  if [[ ${docker_version} != 19.03.0 && ${docker_version} < 19.03.0 ]]; then
    echo "Minimum docker version 19.03.0 is required for using docker buildx: ${docker_version}]"
    exit 1
  fi
}

# This function will build and push the image for all the architectures mentioned in BASEIMAGE file.
build_and_push() {
  docker_version_check

	# Enable execution of multi-architecture containers
	docker run --rm --privileged multiarch/qemu-user-static:${QEMUVERSION} --reset -p yes
  docker buildx create --name img-builder --use
  # List builder instances
  docker buildx ls
  trap "docker buildx ls && docker buildx rm img-builder" EXIT

  os_archs=$(listOsArchs)
  for os_arch in ${os_archs}; do
    splitOsArch "${os_arch}"

    echo "Building / pushing image for OS/ARCH: ${os_arch}..."

    dockerfile_name="Dockerfile"
    if [[ "$os_name" = "windows" ]]; then
      dockerfile_name="windows.Dockerfile"
    fi

    BASEIMAGE=$(getBaseImage "${os_arch}")

    # We only have BASEIMAGE_CORE for Windows images.
    BASEIMAGE_CORE=$(getBaseImage "${os_arch}" "BASEIMAGE_CORE") || true

    # NOTE(claudiub): docker buildx works for Windows images as long as it doesn't have to
    # execute RUN commands inside the Windows image.
    docker buildx build --no-cache --pull --push --platform "${os_name}/${arch}" -t "${IMAGE_TAG}-${suffix}" \
      --build-arg BASEIMAGE="${BASEIMAGE}" --build-arg BASEIMAGE_CORE="${BASEIMAGE_CORE}" \
      --build-arg TARGETARCH="${arch}" --build-arg TARGETOS="${os_name}" --build-arg LDFLAGS="${LDFLAGS}" \
      --build-arg IMAGE_VERSION="${IMAGE_VERSION}" \
      -f "${dockerfile_name}" ..

    # Build and push crd image
    # We always promote to prod from stg registry. So, Check if 'crd' dir from 'charts' exists rather than from 'manifest_staging' dir.
    if find charts/secrets-store-csi-driver/crds -mindepth 1 -maxdepth 1 | read -r; then
      if [[ "$os_name" != "windows" ]]; then
        docker buildx build --no-cache --pull --push --platform "${os_name}/${arch}" -t "${CRD_IMAGE_TAG}-${suffix}" \
        -f crd.Dockerfile ../charts/secrets-store-csi-driver/crds
      fi
    fi
  done
}

# This function will create and push the manifest list for the image
manifest() {
  os_archs=$(listOsArchs)
  # Make os_archs list into image manifest. Eg: 'linux/amd64 windows/amd64/1809' to '${IMAGE_TAG}-linux-amd64 ${IMAGE_TAG}-windows-amd64-1809'
  while IFS='' read -r line; do manifest+=("$line"); done < <(echo "$os_archs" | sed "s~\/~-~g" | sed -e "s~[^ ]*~${IMAGE_TAG}\-&~g")
  docker manifest create --amend "${IMAGE_TAG}" "${manifest[@]}"

  # Create manifest with the crd images
  for os_arch in ${os_archs}; do
    splitOsArch "${os_arch}"
    # Add to manifest if os_arch starts with linux
    if [[ "$os_name" != "windows" ]]; then
      crd_manifest+=$(echo "$os_arch" | sed "s~\/~-~g" | sed -e "s~[^ ]*~driver-crds\-&~g")
    fi
  done
  docker manifest create --amend "${CRD_IMAGE_NAME}" "${crd_manifest[@]}"

  # We will need the full registry name in order to set the "os.version" for Windows images.
  # If the ${REGISTRY} dcesn't have any slashes, it means that it's on dockerhub.
  registry_prefix=""
  if [[ ! $REGISTRY =~ .*/.* ]]; then
    registry_prefix="docker.io/"
  fi
  # The images in the manifest list are stored locally. The folder / file name is almost the same,
  # with a few changes.
  manifest_image_folder=$(echo "${registry_prefix}${IMAGE_TAG}" | sed "s|/|_|g" | sed "s/:/-/")

  for os_arch in ${os_archs}; do
    splitOsArch "${os_arch}"
    docker manifest annotate --os "${os_name}" --arch "${arch}" "${IMAGE_TAG}" "${IMAGE_TAG}-${suffix}"

    # Annotate the crd images
    if [[ "$os_name" != "windows" ]]; then
      docker manifest annotate --os "${os_name}" --arch "${arch}" "${CRD_IMAGE_NAME}" "${CRD_IMAGE_NAME}-${suffix}"
    fi

    # For Windows images, we also need to include the "os.version" in the manifest list, so the Windows node
    # can pull the proper image it needs.
    if [[ "$os_name" = "windows" ]]; then
      BASEIMAGE=$(getBaseImage "${os_arch}")
      # Getting the full OS version from the original image manifest list.
      full_version=$(docker manifest inspect "${BASEIMAGE}" | grep "os.version" | head -n 1 | awk '{print $2}') || true

      # At the moment, docker manifest annotate doesn't allow us to set the os.version, so we'll have to
      # it ourselves. The manifest list can be found locally as JSONs.
      sed -i -r "s/(\"os\"\:\"windows\")/\0,\"os.version\":$full_version/" \
        "${HOME}/.docker/manifests/${manifest_image_folder}/${manifest_image_folder}-${suffix}"
    fi
  done

  echo "Manifest list:"
  docker manifest inspect "${IMAGE_TAG}"
  docker manifest push --purge "${IMAGE_TAG}"
  docker manifest inspect "${CRD_IMAGE_NAME}"
  docker manifest push --purge "${CRD_IMAGE_NAME}"
}

shift
eval "${TASK}"
