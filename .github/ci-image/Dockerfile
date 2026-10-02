# Supply an inspected official archlinux:base digest; no floating default.
ARG ARCH_BASE
FROM ${ARCH_BASE} AS native
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
LABEL org.opencontainers.image.source="https://github.com/sh4869221b/azerlay"
# Full Arch upgrade in each package-install transaction; never partial updates.
RUN pacman -Syu --noconfirm --needed \
      ca-certificates git curl tar gzip zstd base-devel pkgconf \
      gtk4 gtk4-layer-shell gobject-introspection \
    && rm -f /var/cache/pacman/pkg/* \
    && mkdir -p /usr/local/share/azerlay-ci \
    && LC_ALL=C pacman -Q | LC_ALL=C sort > /usr/local/share/azerlay-ci/packages.txt \
    && pkg-config --exists gtk4 gtk4-layer-shell-0 \
    && command -v cc readelf zstd

FROM native AS wayland
RUN pacman -Syu --noconfirm --needed \
      sway dbus grim noto-fonts-cjk noto-fonts-emoji \
    && setcap -r /usr/bin/sway \
    && test -z "$(getcap /usr/bin/sway)" \
    && rm -f /var/cache/pacman/pkg/* \
    && LC_ALL=C pacman -Q | LC_ALL=C sort > /usr/local/share/azerlay-ci/packages.txt \
    && pkg-config --exists gtk4 gtk4-layer-shell-0
