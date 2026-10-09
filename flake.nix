{
  description = "Azerlay pinned Nix CI comparison environments";

  # Source archive and NAR hash are committed in flake.lock. CI never updates it.
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/494ce7fd23ff6a5dff39e1fb11e9b6f2ac74bf25";

  # Only Go comes from this security update; native tools/libraries stay pinned.
  inputs.nixpkgs-go.url = "github:NixOS/nixpkgs/423bc95d0ed475b23dada7d1f9a6a6b66e9bede3";

  outputs = { nixpkgs, nixpkgs-go, ... }:
    let
      system = "x86_64-linux";
      pkgs = import nixpkgs { inherit system; };
      go = (import nixpkgs-go { inherit system; }).go_1_27;
      lib = pkgs.lib;
      baseTools = with pkgs; [
        go pkg-config python3 git bash coreutils findutils gnugrep gnused
        gawk diffutils binutils which zstd cacert fontconfig
      ];
      libraries = with pkgs; [
        gtk4 gtk4-layer-shell glib cairo pango gobject-introspection
      ];
      mkCI = withGUI:
        let
          tools = baseTools ++ lib.optionals withGUI (with pkgs; [ sway-unwrapped grim dbus glib.bin at-spi2-core ]);
          fonts = lib.optionals withGUI (with pkgs; [ noto-fonts-cjk-sans noto-fonts-color-emoji ]);
        in pkgs.mkShell {
          name = if withGUI then "azerlay-ci-test" else "azerlay-ci-build";
          packages = tools;
          buildInputs = libraries ++ fonts;
          GOTOOLCHAIN = "local";
          CGO_ENABLED = "1";
          SSL_CERT_FILE = "${pkgs.cacert}/etc/ssl/certs/ca-bundle.crt";
          # ci-nix-run excludes inherited host tools and GUI configuration.
          AZERLAY_NIX_PATH = lib.makeBinPath (tools ++ [ pkgs.stdenv.cc ]);
          AZERLAY_NIX_FONT_DIRS = lib.concatMapStringsSep ":" (p: "${p}/share/fonts") fonts;
          # Color emoji uses bitmap strikes and needs fontconfig's standard
          # scaling rule; font directories alone produce oversized Pango lines.
          AZERLAY_NIX_FONT_RULES = lib.optionalString withGUI "${pkgs.fontconfig.out}/share/fontconfig/conf.avail/10-scale-bitmap-fonts.conf";
          AZERLAY_NIX_DATA_DIRS = "${pkgs.gsettings-desktop-schemas}/share/gsettings-schemas/${pkgs.gsettings-desktop-schemas.name}:${pkgs.shared-mime-info}/share:${pkgs.gtk4}/share/gsettings-schemas/${pkgs.gtk4.name}:${pkgs.gtk4}/share";
          # Explicit runtime paths avoid the host /etc D-Bus session config and
          # at-spi2-core's wrapper, which otherwise prepends /usr/bin to PATH.
          AZERLAY_NIX_ATSPI = lib.optionalString withGUI "${pkgs.at-spi2-core}";
          AZERLAY_NIX_DBUS = lib.optionalString withGUI "${pkgs.dbus}";
          AZERLAY_NIX_BASH = "${pkgs.bash}/bin/bash";
          AZERLAY_NIX_CI = "1";
          AZERLAY_NIX_CI_ROLE = if withGUI then "test" else "build";
        };
    in {
      devShells.${system} = {
        ci-test = mkCI true;
        ci-build = mkCI false;
        default = mkCI true;
      };
      ciVersions = {
        inherit system;
        go = go.version;
        gtk4 = pkgs.gtk4.version;
        glib = pkgs.glib.version;
        layerShell = pkgs.gtk4-layer-shell.version;
        sway = pkgs.sway-unwrapped.version;
        cairo = pkgs.cairo.version;
        pango = pkgs.pango.version;
        grim = pkgs.grim.version;
        atspi = pkgs.at-spi2-core.version;
        dbus = pkgs.dbus.version;
      };
    };
}
