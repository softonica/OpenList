# Changelog — OpenList Server (workspace)

Semua perubahan pada OpenList Server di workspace ini dicatat di sini.
Format mengikuti [Keep a Changelog](https://keepachangelog.com/), bahasa Indonesia.
(Melacak perubahan workspace di atas upstream `github.com/OpenListTeam/OpenList`.)

## [Unreleased]

### Added

- **TeraBox — login via QR (app TeraBox) sebagai opsi utama; `cookie` jadi
  opsional.** Driver Terabox menjalankan flow QR dari repo
  `mill-master/rclone-terabox` (helper `bin/terabox-login`): minta QR dari
  `https://www.1024terabox.com/ai/index` (`POST /passport/qrcode/get`), user
  scan + konfirmasi di app TeraBox, driver poll `/passport/qrcode/login`
  tiap 2 detik sampai `ndus`, simpan ke field `cookie` (`ndus=<token>`).
  QR state disimpan di addition (`qrcode_state`) dan QR page (`need verify`
  + `data-state`) ditampilkan di UI; save berikutnya mem-poll state hingga
  `ndus` masuk. `Init()`: cookie kosong → mulai QR flow / poll state; cookie
  invalid → QR flow. Field `cookie` sekarang opsional; **cookie-paste tetap didukung** sebagai
  fallback, dan cookie yang kadaluarsa otomatis memicu ulang QR flow saat
  storage di-reload/save. Implementasi: `server/drivers/terabox/login.go`
  (baru), `server/drivers/terabox/login_test.go` (13 unit test, mock
  httptest), `server/drivers/terabox/driver.go` (`Init()` rework),
  `server/drivers/terabox/meta.go`.

- **Workflow GitHub Actions `run-on-actions.yml`** — jalankan server OpenList di
  VM GitHub Actions (ubuntu-latest) dan ekspos via **Cloudflare quick tunnel**
  (URL `*.trycloudflare.com`). Build Go otomatis, restore data dari artifact
  `openlist-data`, backup tiap 5 menit (persistensi antar run), auto-restart
  tiap 6 jam via `schedule` (maksimum runtime hosted runner), trigger manual via
  `workflow_dispatch`. URL tunnel muncul di job summary; login default `admin/admin`.
  Backup data **dienkripsi AES-256** (`openssl -pbkdf2`) bila secret
  `ARTIFACT_KEY` diset (disarankan untuk repo publik agar kredensial storage
  tidak bocor via artifact).

- **Deploy OpenList mod `v4.2.10-custom` ke VPS ranknerd** — binary Linux
  amd64 di-build ulang dari commit `aa92f1ab` dan dijalankan sebagai service
  systemd `openlist.service` dengan autostart; data di `/opt/openlist/data`,
  bind lokal `127.0.0.1:5244`. Domain `rk.gofile.my.id` (A record proxied
  lewat Cloudflare) mengarah ke VPS dengan nginx reverse proxy 80/443 dan
  origin TLS self-signed (zone memakai mode SSL `full`). Setup admin pertama
  diserahkan ke user lewat wizard `/api/public/init/setup`.
- **Deploy OpenList mod `v4.2.10-custom` ke seedbox Pulsed Media**
  (`092606a9a3d92b21.mcx.fi`, hostname `le4-0-78-98cave-johnson.pulsedmedia.com`,
  IP `185.148.0.98`, Debian 12, user `oneindo` **non-root**). Binary
  `server/build/openlist-linux-amd64-rk` (commit `aa92f1ab`) dipasang di
  `/home/oneindo/openlist/openlist` dengan **data di `/home/oneindo/openlist/data`**
  (sqlite + log/temp/bleve di dalamnya). Karena 80/443 diduduki nginx front-proxy
  PMSS dan user tak punya root, OpenList di-bind `127.0.0.1:5244` dan diekspos
  lewat **Caddy** (reverse proxy TLS) di port tinggi `2053`; sertifikat
  **Let's Encrypt** diperoleh otomatis via **DNS-01** memakai token Cloudflare
  (zone `gofile.my.id`) — Caddy v2.11.7 + modul `dns.providers.cloudflare`.
  DNS A `openlist.gofile.my.id` → `185.148.0.98` (DNS-only). Persistensi via
  **systemd user service** (`openlist.service` + `caddy.service`, `Restart=always`,
  `enabled`, linger aktif) → auto-start saat boot & auto-restart saat crash.
  Admin: `admin` / `OpenList@2026!`. URL: `https://openlist.gofile.my.id:2053`.

### Changed

- **Deploy core `v4.2.9` ke `ca.vidku.net`** — binary yang dipakai Gofile
  Desktop (`server/openlist-pixeldrain.exe`) diganti build `v4.2.9-custom`
  (sebelumnya `dev`, build 12 Okt). Data `%APPDATA%\Gofile Desktop\data`
  dipertahankan apa adanya (backup `data.db`, `config.json`, dan binary lama
  di folder `backup-20261026-154645`). Verifikasi: `/ping` lokal dan publik
  `pong`, frontend ter-serve, dan storage `/PD` (Pixeldrain), `/abyss`,
  `/abyss1` (Abyss) tetap load sukses.
- **Core diselaraskan ke upstream OpenList `v4.2.9`** (rilis 25 Okt 2026, dari
  basis commit `940b69f` Juli 2026) — 295 file upstream ikut masuk (161 diubah,
  132 baru, 2 dihapus), sementara seluruh fitur fork tetap dipertahankan:
  driver Abyss & Pixeldrain, role General/Minimax/Pertamax, kuota storage &
  bandwidth harian, stats dashboard, registrasi user, share hardening
  (rate-limit + kuota per share), dan respons HTTP status asli.
  Perubahan upstream yang ikut masuk antara lain: fitur *transfer seed*
  (`SeedSiteURL`, matriks hash, format oss/torrent/cas), media library +
  transcode, `cmd/worker`, penulisan ulang WebDAV lock/prop, S3 multipart,
  serta driver baru (139, 189tv, guangyapan, thunderx, strm,
  onedrive_sharelink, alist_v3). `go.mod` naik ke `go 1.26` /
  toolchain `go1.27.3`. Konflik merge diselesaikan manual di 6 file:
  `internal/bootstrap/data/setting.go`, `internal/bootstrap/data/user.go`,
  `internal/conf/const.go`, `internal/db/db.go`, `internal/net/serve.go`,
  `server/router.go`.
- **Core diselaraskan ke upstream OpenList `v4.2.10`** (rilis 19 Nov 2026) —
  34 commit upstream ikut masuk, seluruh fitur fork tetap dipertahankan.
  Perubahan upstream utama: driver baru **CtFile** (`ctfile`) dan dukungan
  **SharePoint site document library** di OneDrive App; **SSO Feishu**;
  media library scan per-level folder + simpan inkremental + tag TV + progres
  task seed; fix sharing (hitung akses atomik per-IP, sembunyikan path fisik di
  error download); fix stream resume otomatis; fix local cross-device recycle
  bin; fix 115 upload (retry decrypt, multipart); fix quark_open `content_hash`;
  persist token rotasi + WAL SQLite; drop lite build variant. Konflik merge
  diselesaikan manual di `internal/conf/const.go`; test upstream disesuaikan ke
  konvensi fork status HTTP asli.
- **Deploy core `v4.2.10-custom` ke `localhost:5244`** — binary
  `server/openlist-pixeldrain.exe` di-rebuild dari core baru (ldflags
  `-s -w`, version `v4.2.10-custom`, git commit `f6e64d65`), embedded frontend
  disinkronkan dari `web/dist` (11/13). Data `%APPDATA%\Gofile Desktop\data`
  dipertahankan apa adanya (backup binary lama di
  `openlist-pixeldrain-bak-20261119.exe`). Verifikasi: `/ping` `pong`,
  frontend ter-serve dari `dist_dir` (`web/dist`), dan storage `/PD`
  (Pixeldrain), `/abyss`, `/abyss1` (Abyss) tetap load sukses.

### Added

- **Dokumentasi fitur baru: Seed & Transcoding** — `docs/seed.md` (transfer
  seed: format `oss`/`torrent`/`cas`, generate/parse/convert/diagnose,
  rapid upload CAS, auto-generate sidecar saat upload, API & setting, peta
  file) dan `docs/transcoding.md` (FFmpeg edge-transcode: arsitektur
  Master/Worker, protokol HTTP/JSON, keputusan Decide, profil output,
  hardware acceleration, chunked processing, cache, API & setting, cara
  menjalankan worker remote).

### Added

- **Driver storage baru: Abyss (`abyss.to`)** — storage khusus upload video:
  `List` (`GET /v1/resources`), `Put` (streaming multipart ke
  `POST http://up.abyss.to/<apiKey>`, RAM tetap datar untuk video besar),
  `MakeDir` (`POST /v1/folders`), `Move` file & folder
  (`PATCH /v1/files|folders/:id?parentId=`), `Rename` file & folder
  (`PUT /v1/files|folders/:id`), `Remove` (`DELETE /v1/files|folders/:id`),
  dan
  `Link` berupa URL play branded (iframe, `custom_domain` atau
  `play.abyssplayer.com`). Sesuai batasan abyss, upload diserialisasi: hanya
  satu upload dalam satu waktu, upload berikutnya mengantre (batal bila
  request di-cancel). `Copy` tidak didukung (tidak ada API-nya), sehingga
  "copy ke Abyss" antar-storage tetap jalan lewat jalur download+upload.
  Konfigurasi: `api_key`, `email`, `password`, `custom_domain` (opsional),
  `root_folder_id` (opsional). Skema body `POST /v1/folders`
  (`{"name", "parentId"}`) telah diverifikasi terhadap API live.

### Fixed

- **TeraBox: QR yang ditampilkan tidak lagi basi (app menolak "kode
  kadaluarsa").** State QR — termasuk gambar QR — disimpan di addition dan
  sebelumnya di-poll apa adanya. Sesi uuid di server hidup 20+ menit, tapi
  **gambar QR hanya bisa di-scan beberapa menit** (app TeraBox menolaknya
  sebagai expired). Kini setiap state mencatat `created_at`; QR yang lebih
  tua dari 150 detik diterbitkan ulang **sebelum** polling dan dijawab
  dengan halaman QR segar (~1 detik), sementara sesi yang masih segar tetap
  dipertahankan (tidak kembali ke perilaku churn). State lama tanpa
  timestamp dianggap basi dan langsung diperbarui.

- **TeraBox: login QR tidak lagi menggantung "loading terus" padahal app
  sudah konfirmasi sukses.** Jalur window-timeout menyimpan ulang (dan
  membuat UUID baru untuk) sesi QR setiap kali window 90s habis, padahal sesi
  QR TeraBox hidup 20–35+ menit — konfirmasi user jatuh ke sesi yang sudah
  ditinggalkan dan poll selalu menanyakan uuid baru. Kini window-timeout
  **mempertahankan sesi yang sama** (halaman QR yang sama dikirim ulang),
  dan QR baru hanya diterbitkan saat server menyatakan sesi benar-benar mati
  (`code 36/37`). Frontend hanya menampilkan toast "QR diperbarui" bila uuid
  benar-benar berubah.

- **TeraBox: endpoint QR login mengikuti domain dinamis (tidak lagi
  hardcode `1024terabox.com`).** Halaman QR kini diturunkan dari base URL
  driver (`www.terabox.com` default, atau domain region `<prefix>.terabox.com`
  hasil header `Url-Domain-Prefix` saat `errno -6` — mekanisme yang
  didokumentasikan ALIST #7499), dengan override khusus test. Setelah login
  sukses, `base_url` **tidak lagi** ditimpa ke origin halaman QR — domain
  hasil resolusi dipertahankan, sehingga semua API berikutnya memakai domain
  resmi/region yang benar (cookie `ndus` account-scoped, valid lintas alias;
  `1024terabox.com`, `nephobox.com`, `4funbox.com` dan `mirrobox.com` adalah
  alias yang melayani halaman QR identik — terverifikasi via probe).

- **TeraBox: listing storage yang belum login tidak lagi tampak "folder
  kosong".** `getFiles` sebelumnya menelan semua `errno` selain 9000; tanpa
  cookie (login QR belum selesai), API list menjawab `{"errno":-6}` dan UI
  menampilkan folder kosong seolah mount sehat (teramati live di mount
  `/TBtestQR`). Kini `errno != 0` menjadi error `terabox list failed,
  errno: N`; folder kosong yang sah (`errno` 0) tetap tanpa error dan `errno`
  9000 tetap berpesan khusus "not yet available in this area".

- **TeraBox: save storage saat QR-login tidak lagi mati opak di ~2 menit.**
  Window polling di dalam request save (180s, warisan referensi CLI yang
  tidak terikat umur HTTP request) melebihi umur request di jalur jaringan
  nyata — teramati dua kali `context canceled` tepat `2m0s`. Window kini
  **90s** dan cek deadline memperhitungkan durasi long-poll (35s), sehingga
  save selalu menjawab dengan halaman QR baru (`need verify`) sebelum jalur
  mematikan request — loop login QR jadi *self-sustaining*.

- **TeraBox: storage QR-login tidak lagi macet saat QR state kedaluwarsa —
  otomatis minta QR baru.** Poll `/passport/qrcode/login` yang menjawab
  `{"code":36,"msg":"invalid uuid"}` atau `{"code":37,"msg":"uuid expire"}`
  (sesi QR mati, teramati live 24 Nov) sebelumnya menggagalkan `Init()` dengan
  error keras; setiap save/restart mengulang poll atas uuid yang sama yang
  sudah mati, sehingga storage tidak bisa dipulihkan lewat QR. Kini error
  tersebut memicu QR flow baru (`startQRFlow`): QR segar diambil, state baru
  disimpan, dan halaman QR ditampilkan di UI — alur login QR kini
  *self-healing*. Sekalian dikunci kontrak desain "QR = alternatif login,
  cookie = permanen" dengan test e2e baru `TestQrLoginPersistsCookieViaOpFlow`
  (DB in-memory melalui `op.CreateStorage`/`op.UpdateStorage`, pola
  `115_open/driver_test.go`): login QR sukses → `cookie=ndus=…` tersimpan
  permanen di DB dan `qrcode_state` dibersihkan.

- **TeraBox: login QR gagal `{"code":6,"msg":"System error, please try
  again later"}` saat polling** — `pollQRStatus` mengirim
  `POST /passport/qrcode/login` dengan cookie jar kosong (cookie `browserid`
  tidak ikut terkirim) dan tanpa header browser (User-Agent bawaan
  `go-resty/...`), sehingga endpoint passport TeraBox menolak setiap poll
  secara instan — bukan karena QR expired (diverifikasi langsung ke endpoint
  asli via probe: bentuk request benar diterima dan di-long-poll ~26 detik
  lalu dijawab `39` pending; bentuk lama tetap ditolak `code 6` beberapa
  detik kemudian pada sesi QR yang sama). Poll kini membawa cookie
  `browserid` dari QR state + header lengkap ala browser (User-Agent Chrome,
  Accept, Accept-Language, Origin, Referer, X-Requested-With), timeout request
  naik 15s → 35s karena endpoint mem-*long-poll* request yang diterima
  (nilai yang dipakai referensi `bin/terabox-login`), dan token challenge
  `v`/`vcode` dari reply kini disimpan ke state dan di-echo pada poll
  berikutnya, mengikuti referensi.

- **Offline download qBittorrent/Transmission: transfer tidak lagi gagal
  dengan `http request failure, err:Get` untuk sumber magnet** — saat policy
  `upload_download_stream`, `TransferTask.Run()` sebelumnya mencoba mengunduh
  ulang dari URL sumber (`GET` ulang ke `t.Url`). Untuk URL non-HTTP seperti
  `magnet:?xt=…`, GET tersebut pasti gagal (unsupported protocol scheme) dan
  task gagal walaupun file sudah terunduh penuh oleh qBittorrent. Kini jalur
  stream hanya dipakai untuk URL `http(s)`; selain itu transfer membaca file
  lokal yang sudah ada di `SrcActualPath`. Ditambah test unit
  `internal/offline_download/tool/transfer_test.go`.
- **Integrasi qBittorrent: download URL magnet tidak lagi gagal di
  qBittorrent ≥ 5.2.0** — `AddFromLink` (`pkg/qbittorrent/client.go`)
  salah menangani kode status HTTP: `204 No Content` (respons sukses
  sejak qBittorrent 5.2.0) justru dilaporkan sebagai error, sementara
  status kegagalan (`400`/`403`/`415`) dikira sukses. Kini sukses
  diakui dalam tiga bentuk: `204` (≥ 5.2.0), `200` + body `Ok.`
  (< 5.2.0), dan `200` + payload JSON `success_count ≥ 1` (≥ 5.3.0,
  WebAPI 2.14+); status lain (termasuk `409` duplikat) dikembalikan
  sebagai error dengan kode status. Ditambah test unit
  `pkg/qbittorrent/client_test.go` (fake WebUI, 7 kasus).
- **Driver Abyss: fix mount gagal `abyss API 404: Folder not found`** —
  form "add storage" otomatis mengisi `root_folder_id` dengan `/` (dari
  `DefaultRoot`), lalu dikirim sebagai `folderId=/` yang ditolak abyss.
  Kini   `DefaultRoot` dikosongkan dan nilai `""/"/"/"root"` dinormalisasi
  ke root abyss (tanpa parameter `folderId`), sehingga storage yang sudah
  terlanjur tersimpan dengan `/` ikut sembuh tanpa perlu dihapus-buat
  ulang.
- **Driver Abyss: copy banyak file tidak lagi gagal saat paralel** —
  `Put` sebelumnya fail-fast bila ada upload bersamaan, padahal task
  copy OpenList jalan beberapa file paralel (mengikuti setting global).
  Kini upload mengantre lewat slot tunggal seproses (menunggu giliran,
  batal bila request di-cancel), jadi copy 5 file paralel tetap terkirim
  satu per satu sesuai batasan abyss.
- **Driver Abyss: upload ke subfolder tidak lagi mendarat di root** —
  endpoint upload abyss (`up.abyss.to`) tidak punya parameter folder,
  sehingga file yang di-upload ke folder terpilih selalu masuk ke root
  akun. Kini `Put` memindahkan file hasil upload ke `dstDir` lewat
  `PATCH /v1/files/:id?parentId=` setelah upload; `Move` dan `Rename`
  (file & folder) juga diimplementasikan, jadi pindah/rename dari UI
  ikut berfungsi (terverifikasi terhadap API live). Sekaligus: retry
  otomatis saat token JWT kedaluwarsa (401) kini tetap membawa body JSON
  request, dan pengecekan context pada antrean upload dibuat
  deterministik.
- **Driver Abyss: hapus file/folder sebelumnya tidak pernah berhasil** —
  `Remove` memakai `DELETE /v1/resources/:id` yang ternyata bukan route
  delete: abyss membalas 200 dengan listing tetapi tidak menghapus apa
  pun (berlaku juga untuk file `ready`). Kini memakai
  `DELETE /v1/files/:id` (file) dan `DELETE /v1/folders/:id` (folder)
  yang benar-benar menghapus (204). Video yang masih encoding memang
  ditolak server (`404 "This file cannot be deleted"`); pesan itu kini
  diteruskan sebagai error yang jelas dengan petunjuk coba lagi setelah
  video `ready`.
- **Driver Abyss: listing folder dipotong di 25 item** — `GET
  /v1/resources` default 25 item/halaman dan driver mengabaikan
  `pageToken`; folder dengan >25 isi tampil tidak lengkap (root akun
  pengguna berisi ribuan file). `List` kini memakai `maxResults=100` dan
  mengikuti `pageToken` sampai habis.
- **Driver Abyss: file hasil listing tidak bisa dibuka/di-play** — item
  dari `GET /v1/resources` tidak membawa `slug`/URL dan `GET
  /v1/files/:id` tidak memuat field URL, sehingga `Link` selalu gagal
  untuk file yang muncul dari listing (bukan hasil upload sesi berjalan).
  Play slug ternyata sama dengan file id; `List`/`Link` kini selalu
  membangun URL play branded dari id sehingga semua file bisa dibuka.
- **Driver Abyss: buka folder root 9.898 file kini instan (dulu ±55 detik)**
  — `GET /v1/resources` mentok 100 item/halaman tanpa paginasi
  offset/nomor (terverifikasi ke API live), dan token halaman harus
  diambil berurutan, sehingga root (±100 halaman × ±0,5 dtk) selalu
  di-walk penuh ±55 detik setiap cache kedaluwarsa. Kini hasil walk
  penuh di-cache di memori driver 24 jam (buka ulang ±0,04 dtk);
  refresh manual tetap walk live. Tambahan: driver implementasi
  `driver.Getter` — resolve satu objek via `GET /v1/files|folders/:id`
  (±0,4 dtk) berdasar id yang diingat dari listing, sehingga buka
  subfolder, resolve link download, dan baca readme tidak lagi
  memicu walk ulang root saat cache core kedaluwarsa (fallback ke
  walk bila path belum pernah di-list). Cache driver diinvalidasi
  setiap mutasi (upload/mkdir/move/rename/remove). Setting
  `cache_expiration` storage `/abyss` disarankan 1440 menit (24 jam).

- **Remote upload (offline download) SimpleHttp mendukung link Google Drive**
  (mis. `https://drive.usercontent.google.com/download?id=...&export=download`,
  termasuk link `drive.google.com/file/d/<id>/view` yang otomatis
  dinormalisasi). File besar/berisiko yang dibalas halaman "virus scan
  warning" kini otomatis dikonfirmasi (form replay POST dengan parameter
  `confirm`/`uuid`), dan mode stream upload langsung memakai ranged GET untuk
  mendeteksi ukuran. Nama file diambil dari `Content-Disposition`; bila
  tidak ada, fallback `google-drive-<fileId>`. Halaman HTML Google tanpa form
  konfirmasi (file privat/perlu login) menghasilkan error jelas, bukan file
  HTML korup; respons HTML dari host lain tetap disimpan seperti perilaku
  lama.
- **SimpleHttp: dukungan file besar & koneksi tidak stabil** — setiap task
  download kini memakai cookie jar sendiri (sesi download Google yang butuh
  cookie antar redirect/interstitial berfungsi), download otomatis
  **dilanjutkan (resume) via HTTP Range** saat koneksi putus di tengah jalan
  (maks 10 percobaan, backoff eksponensial; server yang mengabaikan Range
  otomatis diulang dari awal), respons chunked tanpa `Content-Length` tetap
  didownload penuh dengan total bytes dilaporkan setelah selesai, dan
  progress tetap akurat saat resume.
- **SimpleHttp: fix remote upload Google Drive tersimpan 2.4KB** — saat
  Google menjawab konfirmasi download dengan interstitial LAGI (rate-limit /
  uuid sekali-pakai basi, kadang berlabel `application/octet-stream` +
  `Content-Disposition` sehingga lolos dari deteksi Content-Type), halaman
  HTML malah tersimpan sebagai "file" 2.4KB dengan nama benar. Kini:
  konfirmasi diulang otomatis maksimal 5 ronde (uuid segar tiap ronde), body
  respons host Google di-sniff (awalan `<!DOCTYPE html`/`<html` → dianggap
  interstitial walau Content-Type binary), body yang berakhir lebih pendek
  dari ukuran umumkan otomatis di-resume (bukan dianggap selesai), dan HTML
  Google tanpa form konfirmasi (file privat / kuota habis) gagal dengan
  pesan jelas alih-alih menyimpan file rusak.
- **SimpleHttp: fix mode "Download stream direct upload"
  (`upload_download_stream`) untuk Google Drive** — di mode ini transfer
  task me-fetch ulang URL asal secara mandiri, sehingga halaman konfirmasi
  Google (virus-scan warning) ikut ter-stream sebagai file 2.4KB yang rusak
  meski konfirmasi sudah ditangani. Kini URL yang diserahkan ke transfer
  task adalah URL direct hasil resolusi (sudah membawa parameter
  `confirm`/`uuid`), sesuai alur resmi Google Drive API untuk file besar.

- **Role Manager (permission per role global)** — admin kini bisa mengatur
  permission bitmask secara global per role (General, Guest, Minimax,
  Pertamax), bukan hanya per user. Permission efektif user = **bit user OR
  bit role** (union; role hanya bisa menambah hak, tidak pernah mencabut hak
  individual). Contoh: centang "WebDAV read" di role Minimax → semua user
  Minimax bisa akses WebDAV.
  - Model baru `model.Role` (`internal/model/role.go`) — tabel `roles`
    (id = konstanta role, name, permission), di-AutoMigrate di
    `internal/db/db.go`; seed 4 role terkelola saat bootstrap
    (`internal/bootstrap/data/role.go`).
  - Registry in-memory `model.SetRolePermissions` / `model.RolePermission`;
    ke-16 method `User.CanXxx()` (termasuk `CanWebdavRead/Manage`,
    `CanFTPAccess/Manage`, dll.) kini meng-OR-kan mask role → berlaku
    instan untuk semua checkpoint (WebDAV, FTP/SFTP, fs write, share,
    archive, offline download, MCP) tanpa invalidasi cache user.
  - `User.EffectivePermission()` — `/api/me` kini mengembalikan permission
    efektif agar UI menyesuaikan hak aktual user.
  - API admin baru: `GET /api/admin/role/list`, `GET /api/admin/role/get`,
    `POST /api/admin/role/update` (hanya `permission` yang bisa diubah;
    role ADMIN tidak dapat dikelola).
  - Pre-check WebDAV auto-register LDAP kini memperhitungkan mask role
    GENERAL (`server/webdav.go` tryLogin).

- **Role bandwidth tier** — 2 role baru `MINIMAX` (3) & `PERTAMAX` (4) di
  `internal/model/user.go`. Role-role ini hanya berbeda di kuota bandwidth
  harian share (permissi/akses sama seperti General). Kuota dihitung per
  **owner** (bukan per-share lagi): download dari share manapun milik user itu
  dihitung ke pool harian sesuai role-nya.
  - Setting baru (group GLOBAL, PRIVATE, default): `bandwidth_daily_guest`
    (3 GB), `bandwidth_daily_general` (5 GB), `bandwidth_daily_minimax`
    (50 GB), `bandwidth_daily_pertamax` (500 GB). Admin = unlimited. 0 = unlimited.
  - `internal/sharing/quota.go` key diubah per-owner (ownerKey + hari UTC).
  - `server/handles/sharing.go` helper `roleBandwidthLimit(user)` mapping
    role→setting; `SharingDown` pakai owner key + limit role. Setting lama
    `share_daily_quota_bytes` tidak dipakai lagi (tetap terdaftar, tanpa efek).
  - Terverifikasi runtime: demote user ke pertamax + set 1 byte → download 429.

- **Storage quota per user** — batasan alokasi storage per user: default
  per-role (general 10 GB, minimax 100 GB, pertamax 1 TB) + override per-user.
  Admin & guest tidak dibatasi. Pemakaian dihitung via counter `storage_used`
  per-user (naik saat upload, turun saat hapus; sadar overwrite = newSize −
  oldSize).
  - Field baru di `model.User`: `storage_quota` (`-1` = pakai default role;
    `0` = unlimited; `>0` = bytes eksplisit) & `storage_used` (counter bytes,
    di-increment atomik di kolom, bukan rewrite row).
  - Setting baru (group GLOBAL, PRIVATE, default): `storage_quota_general`
    (10 GB), `storage_quota_minimax` (100 GB), `storage_quota_pertamax`
    (1 TB). `0` = unlimited. Konstanta di `internal/conf/const.go`.
  - Helper: `fs.EffectiveStorageQuota(user)` + `fs.checkStorageQuota(ctx,size)`
    (`internal/fs/storage_quota.go`); tracking `op.AddStorageUsage` /
    `op.SubStorageUsage` / `op.RecomputeUserStorage`
    (`internal/op/storage_quota.go`). Error baru `errs.StorageQuotaExceeded`.
  - Enforcement di 3 entry-point upload (`fs/put.go`: `putAsTask`,
    `putDirectly`, `getDirectUploadInfo`) — reject cepat sebelum cache/upload.
    Tracking increment di `op.Put`, decrement di `op.Remove`.
  - Endpoint baru: `GET /api/me/storage` (pemakaian & quota sendiri),
    `POST /api/admin/user/storage/recompute?id=` (recompute via walk BasePath
    — koreksi drift). Field `storage_quota` ikut di form create/update user.
  - Test unit `internal/fs/storage_quota_test.go` — 17 case (override vs role
    default, admin unlimited, under/over quota, overwrite delta). Semua lulus.
  - Catatan arsitektur: logic baca-setting ditaruh di package `fs` (bukan `op`)
    karena `internal/setting` sudah meng-impor `internal/op` (anti import-cycle).

- **Admin Analytics Dashboard** — statistik platform untuk admin, pakai data
  yang benar-benar tersedia di OpenList (bukan PRD generik yang asumsikan tabel
  `Files`/Redis):
  - **Model** baru `internal/model/daily_stats.go` (`DailyStats`: date PK,
    bandwidth_bytes, downloads, rate_limit_hits, quota_hits) + didaftarkan ke
    `AutoMigrate` (`internal/db/db.go`) → tabel `x_daily_stats`.
  - **Kolektor** `internal/stats/stats.go`: counter in-memory harian
    (RecordDownload/RecordRateLimitHit/RecordQuotaHit), reset UTC midnight,
    di-flush ke DB tiap 10 menit (`stats.Init()` dipanggil di
    `internal/bootstrap/db.go` setelah `db.Init`).
  - **DB helper** `internal/db/stats.go`: UpsertDailyStats, GetDailyStatsRange,
    CountUsers, CountSharings, SumShareAccesses, TopShares.
  - **Wiring**: `SharingDown` rekam setiap download (bandwidth+count) +
    quota hit; middleware `ShareIPRateLimit` rekam rate-limit hit.
  - **Endpoint admin** `server/handles/stats.go` + route group `/admin/stats`
    (`AuthAdmin`): `GET /overview`, `GET /traffic?days=N`, `GET /top-shares?limit=N`.
- **Endpoint publik quota share** `GET /sq/:sid` (`handles.SharingQuota`) —
  mengembalikan `{used_bytes, limit_bytes}` owner share untuk indikator
  bandwidth di halaman share. Ditambah `sharing.TodayUsage(ownerKey)`.

### Fixed

- **Startup crash `UNIQUE constraint failed: x_roles.name`** — `model.Role.ID`
  (PK = konstanta role, `GENERAL == 0`) ditandai `autoIncrement:false` karena
  GORM meng-skip nilai PK nol saat INSERT, sehingga baris "general" tersimpan
  dengan id auto (1) dan boot berikutnya gagal create. `initRoles()` kini
  self-healing: baris legacy yang id-nya bergeser dipindahkan ke id konstanta
  yang benar berdasarkan nama unik (`db.GetRoleByName`, `db.SetRoleId` baru).

- **Race condition quota & rate-limit** — `internal/sharing/quota.go` diubah ke
  `ReserveDownload` atomic (mutex, check-and-record); `server/middlewares/
  share_limit.go` check-and-increment dibungkus mutex. Menghilangkan TOCTOU yang
  bisa over-admit di bawah konkurensi.

- **Driver storage Pixeldrain** baru di `drivers/pixeldrain/` (4 file: `driver.go`,
  `meta.go`, `types.go`, `util.go`):
  - **Mode A (free tier)** — `List`/`Link`/`Put`/`Remove`/`MakeDir` via Pixeldrain
    file/list API. File akun ditampilkan datar; "list" akun ditampilkan sebagai
    folder virtual (MakeDir membuat list baru).
  - **Mode B (filesystem, paid-ready)** — `List`/`Link`/`Put`/`Remove`/`MakeDir`/
    `Rename`/`Move` via `/filesystem` API dengan direktori asli. Aktif otomatis
    saat akun berlangganan.
  - **Auto-detect tier** dari `GET /user` (`subscription.name`); dapat dipaksa
    lewat config `mode` (`auto`/`free`/`filesystem`).
  - Config: `api_key` (wajib), `root_folder_path`, `mode`.
- Registrasi driver di `drivers/all.go` (blank-import).

### Added

- **Hardening route share publik** (`/sd/` `/sad/`) — dua lapis proteksi anti-abuse,
  pakai in-memory cache (bukan Redis), default **nonaktif** (0):
  - **Rate-limit per-IP** (`server/middlewares/share_limit.go`) — middleware
    `ShareIPRateLimit()` membatasi request per IP per menit (fixed 1-min window).
    Dikonfigurasi via setting `share_ip_rate_limit` (request/menit, 0 = disabled).
    Melebihi batas → HTTP `429 Too Many Requests`.
  - **Kuota bandwidth harian per share** (`internal/sharing/quota.go`) —
    `QuotaExceeded`/`RecordDownload` melacak total byte diunduh per share per
    hari UTC (reset harian, TTL cache 25 jam). Dikonfigurasi via setting
    `share_daily_quota_bytes` (byte, 0 = unlimited). Ukuran file dihitung di awal
    download via `op.Get` (cached). Melebihi kuota → HTTP `429`.
  - Dua setting baru didaftarkan di `InitialSettings` (group GLOBAL, PRIVATE):
    `share_ip_rate_limit` (default 0), `share_daily_quota_bytes` (default 0).
  - Konstanta baru di `internal/conf/const.go`: `ShareIPRateLimit`,
    `ShareDailyQuotaBytes`.
  - Middleware dipasang di `server/router.go` untuk semua route `/sd/:sid` &
    `/sad/:sid` (GET + HEAD).

### Changed

- **Admin dikecualikan dari rate-limit download** — akun admin (role `ADMIN`)
  kini bypass kedua limiter download di semua jalur (WebDAV, API, share direct,
  file-manager direct); user non-admin tetap tunduk pada batas yang
  dikonfigurasi (`max_client_download_speed` & `max_server_download_speed`).
  - Helper `stream.ServerDownloadLimiterFor(ctx)` (`internal/stream/limit.go`)
    mengembalikan `nil` (unlimited) untuk admin, limiter global untuk non-admin.
    Dipakai di `server/common/proxy.go` (transparent proxy) & `internal/stream/
    util.go` (`GetRangeReaderFromLink`, path concurrency + range) menggantikan
    referensi langsung ke `stream.ServerDownloadLimit` (limiter sisi server,
    OpenList→cloud).
  - Middleware `middlewares.DownloadRateLimiter` (`server/middlewares/limit.go`)
    kini resolve admin dari 3 sumber via `resolveAdminRateLimit`: (1) user
    terautentikasi `conf.UserKey` (WebDAV/API), (2) **pemilik share** lewat
    `conf.SharingIDKey`→`op.GetSharingById` untuk route `/sd/` `/sad/` (admin-
    owned share = tanpa limit), (3) token (master/JWT) di header Authorization
    untuk route publik `/d/` `/p/`. Saat admin terdeteksi, middleware skip
    membungkus response writer DAN menstempel `conf.RateLimitUserKey` ke context
    agar `ServerDownloadLimiterFor` (di common.Proxy) juga bypass.
  - Context key baru `conf.RateLimitUserKey` (`internal/conf/const.go`) terpisah
    dari `UserKey` agar tidak mencemari semantik autentikasi di route publik.
  - Penanganan `nil` limiter sudah benar: `RateLimitReader`/`RateLimitWriter`
    hanya memanggil `WaitN` saat `Limiter != nil`.

- **Naikkan pool koneksi HTTP upstream** — `internal/net/serve.go` `NewHttpClient`
  menyetel `MaxIdleConnsPerHost: 20` (sebelumnya default Go = 2). Mengurangi
  connection starvation/churn saat banyak download concurrent ke host cloud
  yang sama (gejala: salah satu file sangat lambat).

- **`server/handles/sharing.go` `SharingDown`** — sisipan blok cek+catat kuota
  byte setelah `GetStorageAndActualPath`, sebelum serving (proxy/redirect). Hanya
  aktif saat `share_daily_quota_bytes > 0` (tidak ada overhead saat disabled).

- **Quota bandwidth berlaku juga untuk file manager** — `server/handles/down.go`
  (`Down` & `Proxy`) memanggil `enforceDownloaderQuota`: download via `/d/` &
  `/p/` diattribusi ke **downloader** (resolve dari token, soft fallback guest)
  dan masuk pool harian role yang **sama** dengan share download
  (key `<user_id>:<day>`, pool gabungan). Helper baru `downloaderUser` &
  `enforceDownloaderQuota`. Mencegah loophole user menghindari quota via direct
  link `/d/` ber-sign. Dashboard stats kini juga mencakup download file manager.

- **Kelola admin dari UI** — `server/handles/user.go`:
  - `UpdateUser`: blok `"role can not be changed"` dihapus → role user bisa
    diubah lewat API (sebelumnya satu-satunya cara = edit DB manual).
  - `CreateUser`: izin membuat user dengan role admin (blok dikurangi jadi hanya
    guest yang tidak bisa dibuat).
  - Memungkinkan frontend menambah field Role di form edit user. Endpoint tetap
    dilindungi `AuthAdmin` (hanya admin yang bisa memanggil).

- **Proxy Pixeldrain tidak lagi dipaksa otomatis** (`drivers/pixeldrain/driver.go`,
  `meta.go`) — `Config()` tidak lagi menyetel `OnlyProxy` berdasarkan tier akun.
  Toggle `web_proxy` (download) kini muncul di halaman Manage Storage dan bisa
  dinyalakan/dimatikan user seperti driver lain. Catatan: akun **free**
  memblokir hotlinking, jadi untuk akun free sebaiknya proxy dinyalakan;
  tanggung jawab ada di user. Deteksi tier (`paid`) tetap dipakai hanya untuk
  auto-detect mode API (free vs filesystem).

### Catatan

- Didasarkan pada spec `pixeldrain-driver-plan.md` + pengujian API Pixeldrain
  tanggal 2026-08-02 (akun free, semua operasi CRUD + list terverifikasi;
  filesystem API 403 pada free tier).
- Frontend (`web/`) sudah disiapkan: label i18n `Pixeldrain` ditambahkan ke
  `src/lang/{en,id}/drivers.json` (block field + display name).
- Terverifikasi: `go build ./...`, `go vet ./drivers/pixeldrain/`, `gofmt` bersih.
- **Uji runtime** (2026-08-02): driver berfungsi penuh Mode A — upload/list/link/remove
  round-trip sukses via server OpenList berjalan (port 5245, akun free). Mode B
  (filesystem/paid) sudah ter-implementasi, siap aktif saat akun berlangganan.
- **Proxy adaptif:** `OnlyProxy` di-set dinamis di `Config()` — `true` untuk akun
  **free** (bypass blok hotlink via proxy ber-API-key), `false` untuk akun
  **paid/Pro** (hotlinking diizinkan → direct download, lebih efisien). Deteksi
  dari `GET /user` (`subscription.name`).
  - **Update 2026-08-08:** proxy dipaksa dihapus; sekarang user mengatur
    `web_proxy` sendiri dari Manage Storage (lihat entri Changed).
- **Update role GENERAL (id=0) selalu gagal** � dua akar masalah zero-value
  pada role dengan id konstanta 0: (1) `UpdateRoleReq.ID uint` bertag
  `binding:"required"` menolak nilai 0 (validator menganggap kosong) ? diganti
  pointer `*uint`; (2) `db.UpdateRole` memakai `gorm.Save()` yang memperlakukan
  PK nol sebagai CREATE ? bentrok `UNIQUE x_roles.id` ? diganti UPDATE eksplisit
  per kolom (`name`, `permission`) berdasarkan `WHERE id`. Terverifikasi via
  API: update id=0 sukses, perubahan permission tersimpan.
- **Setting `register_default_role` (Site)** � role bawaan untuk user yang
  berhasil registrasi otomatis (SSO auto-register / LDAP) kini bisa diatur
  admin lewat Settings ? Site (pilihan: General / Minimax / Pertamax,
  default General). Sebelumnya hardcoded `Role: 0`; nilai tidak valid akan
  jatuh kembali ke General.
- **Self-registration (`POST /api/auth/register`)** � endpoint publik baru,
  di-gate oleh setting `register_enabled` (Site, default nonaktif). User baru
  mendapat `register_default_role`, password di-hash salted, username
  `admin`/`guest` dicadangkan, duplikat ditolak (409), dan respons langsung
  berisi token agar client lanjut signed-in.
- **Folder pribadi otomatis untuk user registrasi** � setting per-role baru
  `role_base_dir_*` (dikelola di Role Manager, field "Base folder"). Saat
  registrasi sukses, server membuat folder `<username>-<userID>` di dalam
  base folder role tersebut dan menjadikannya `base_path` user. Gagal buat
  folder tidak menggagalkan registrasi (fallback `/` + log warning).
- **Sinkronisasi status izin Users vs Role Manager** � response
  `/api/admin/user/list|get` kini menyertakan `effective_permission`
  (mask user OR mask role), sehingga titik izin di halaman Users mencerminkan
  kemampuan riil user (termasuk izin dari Role Manager), bukan hanya mask
  individunya. Field `permission` tetap seperti aslinya untuk editing.
- **Upload/API path absolut tak lagi nested senyap** � untuk user dengan
  base_path non-root, path yang sudah memuat base directory-nya (mis.
  `/GD-gdkonoha/...` untuk user ber-base `/GD-gdkonoha/Subrek/testuser-9`)
  kini DITOLAK dengan pesan jelas ("path already contains your base
  directory, send a path relative to your base") alih-alih digabung nested
  dan menyimpan file di lokasi salah. Path relatif & traversal (403) tak
  berubah; user base "/" (admin) tak terdampak. Unit test JoinBasePath
  ditambahkan.
- **API kini mengembalikan HTTP status code asli** � `ErrorResp`/
  `ErrorStrResp` sebelumnya selalu HTTP 200 (kode hanya di body JSON),
  menyebabkan sklien/skrip/monitoring salah menyimpulkan sukses. Sekarang
  401/403/404/409/429/500 benar sebagai status HTTP; format body JSON
  (code/message/data) TIDAK berubah. Catatan migrasi klien eksternal: cek
  status HTTP atau tetap baca body seperti biasa. MCP/S3/WebDAV/`/ping` tak
  berubah (sudah pakai protokol masing-masing).
- **Kuota storage kini update realtime & bebas dobel-hitung** �
  (1) `/api/me/storage` dan pengecekan kuota upload membaca `storage_used`
  segar dari DB (salinan user di cache memori tidak lagi membuat statistik
  macet); (2) counter cache di-refresh in-place setiap Add/Sub usage;
  (3) penghitungan pengurangan dipindah ke `fs.remove` dengan ukuran
  rekursif (menghapus folder kini mengurangi total isi foldernya), dan hook
  lama di `op.Remove` dihapus � memperbaiki dobel-pengurangan saat delete
  file, salah hitung pada overwrite, dan move antar-storage. Upload/delete
  file maupun folder terverifikasi simetris (+N / -N).
- **Usage bandwidth per-user di halaman Statistic** � `/api/me/storage`
  kini juga mengembalikan `bandwidth_used_bytes` (hari ini, UTC) dan
  `bandwidth_limit_bytes` (batas harian role). Pencatatan bandwidth kini
  SELALU dilakukan per user (sebelumnya hanya saat limit di-set), tetap
  dengan dedupe per unduhan logis; enforcement limit tidak berubah. Halaman
  Statistic menambah kartu & progress bar bandwidth harian.
