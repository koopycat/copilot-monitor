{pkgs, ...}: {
  languages.go = {
    enable = true;
    package = pkgs.go_1_26;
  };

  enterShell = ''
    echo "go $(go version | cut -d' ' -f3)"
    echo "→ just all"
  '';
}
