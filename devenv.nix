{pkgs, ...}: {
  languages.go = {
    enable = true;
    # Go 1.26.4 contains GO-2026-5856. Remove this override once nixpkgs ships
    # Go 1.26.5 or newer in go_1_26.
    package = pkgs.go_1_26.overrideAttrs (_old: rec {
      version = "1.26.5";
      src = pkgs.fetchurl {
        url = "https://go.dev/dl/go${version}.src.tar.gz";
        hash = "sha256-SVvkvIcXasVnOS5bQRar2YRm0z17SdQedkzMaXay3EI=";
      };
    });
  };

  languages.javascript = {
    enable = true;
    package = pkgs.nodejs_24;
    pnpm = {
      enable = true;
      # pnpm_11 from the locked nixpkgs snapshot is 11.9.0, which crashes with
      #   [ERROR] Cannot use 'in' operator to search for 'integrity' in undefined
      # whenever a package.json is present (a known pnpm 11.9.0 bug). Pin to
      # 11.12.0, which matches the `packageManager` field in package.json and is
      # free of the bug. Drop this override once the devenv-nixpkgs snapshot
      # ships pnpm >= 11.10.0.
      package = pkgs.pnpm_11.overrideAttrs (_old: {
        version = "11.12.0";
        src = pkgs.fetchurl {
          url = "https://registry.npmjs.org/pnpm/-/pnpm-11.12.0.tgz";
          hash = "sha256-HCvxCNdnuXY1PCwemtFNJAzruZ1L702Tp/Gp0Q2luBc=";
        };
      });
    };
  };

  packages = with pkgs; [
    pre-commit
    gotools
    gitleaks
    trufflehog
  ];

  enterShell = ''
    echo "go $(go version | cut -d' ' -f3)"
    echo "node $(node --version)"
    echo "pnpm $(pnpm --version)"
    echo "pre-commit $(pre-commit --version | cut -d' ' -f2)"
    echo "→ just all"
  '';
}
