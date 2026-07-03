{pkgs, ...}: {
  languages.go = {
    enable = true;
    package = pkgs.go_1_26;
  };

  languages.javascript = {
    enable = true;
    package = pkgs.nodejs_24;
  };

  enterShell = ''
    echo "go $(go version | cut -d' ' -f3)"
    echo "node $(node --version)"
    echo "→ just all"
  '';
}
