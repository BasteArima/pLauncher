{
  description = "pLauncher — a private, Steam-like launcher for your local game library";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" ];
      forAll = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
      # Должна совпадать с info.productVersion в game-launcher/wails.json (это проверяет CI).
      # Не читаем wails.json через builtins.readFile: с «ленивыми» деревьями исходников
      # (Determinate Nix) это ломает вычисление flake, скачанного через github:.
      version = "1.0.0";
    in
    {
      packages = forAll (pkgs:
        let
          # Фронтенд (Svelte + Vite) собирается отдельно и встраивается в бинарник (go:embed frontend/dist)
          frontend = pkgs.buildNpmPackage {
            pname = "plauncher-frontend";
            inherit version;
            src = ./game-launcher/frontend;
            npmDepsHash = "sha256-Z9VIM/LwGA93mHNUxjncHPLA2Cll24+23SHD45T/o+g=";
            installPhase = ''
              runHook preInstall
              cp -r dist $out
              runHook postInstall
            '';
          };
        in
        rec {
          plauncher = pkgs.buildGoModule {
            pname = "plauncher";
            inherit version;
            src = ./game-launcher;
            vendorHash = "sha256-GNi9JGcr0nvOgFG7l/6wqZtYyF5cEjRrLIJuInEbYKc=";

            # То же, что делает `wails build`: теги desktop/production, WebKitGTK 4.1
            tags = [ "desktop" "production" "webkit2_41" ];
            ldflags = [
              "-s" "-w"
              "-X main.appVersion=${version}"
              "-X main.updateRepo=BasteArima/pLauncher"
            ];

            nativeBuildInputs = with pkgs; [ pkg-config wrapGAppsHook3 copyDesktopItems ];
            buildInputs = with pkgs; [ gtk3 webkitgtk_4_1 glib-networking ];

            # Загрузке Go-модулей фронтенд не нужен
            overrideModAttrs = _: { preBuild = ""; };

            preBuild = ''
              rm -rf frontend/dist
              cp -r ${frontend} frontend/dist
              chmod -R u+w frontend/dist
            '';

            # Тестам нужны сеть и временные каталоги с особыми путями — гоняются в обычном CI
            doCheck = false;

            postInstall = ''
              mv $out/bin/game-launcher $out/bin/pLauncher
              install -Dm644 build/appicon.png $out/share/pixmaps/plauncher.png
            '';

            preFixup = ''
              gappsWrapperArgs+=(
                # xdg-open — запуск игр без собственного исполняемого файла (html, swf, …)
                --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.xdg-utils ]}
                # Пустое окно на части видеокарт (NVIDIA) с новым рендерером WebKitGTK
                --set-default WEBKIT_DISABLE_DMABUF_RENDERER 1
              )
            '';

            desktopItems = [
              (pkgs.makeDesktopItem {
                name = "plauncher";
                desktopName = "pLauncher";
                comment = "Your local game library";
                exec = "pLauncher";
                icon = "plauncher";
                categories = [ "Game" ];
              })
            ];

            meta = with pkgs.lib; {
              description = "A private, Steam-like launcher for your local game library";
              homepage = "https://github.com/BasteArima/pLauncher";
              license = licenses.gpl3Only;
              mainProgram = "pLauncher";
              platforms = platforms.linux;
            };
          };
          default = plauncher;
        });

      apps = forAll (pkgs: {
        default = {
          type = "app";
          program = "${self.packages.${pkgs.stdenv.hostPlatform.system}.default}/bin/pLauncher";
        };
      });

      # nix develop: всё для `wails dev` / `wails build -tags webkit2_41`
      devShells = forAll (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [ go nodejs wails pkg-config gtk3 webkitgtk_4_1 ];
        };
      });
    };
}
