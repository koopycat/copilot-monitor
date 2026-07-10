{pkgs, ...}: {
  languages.go = {
    enable = true;
    package = pkgs.go_1_26;
  };

  languages.javascript = {
    enable = true;
    package = pkgs.nodejs_24;
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
    echo "pre-commit $(pre-commit --version | cut -d' ' -f2)"
    echo "→ just all"
  '';
}
