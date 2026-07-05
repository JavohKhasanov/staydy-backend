# CI/CD sozlash (GitHub Actions → serverga avto-deploy)

Har repoda `.github/workflows/deploy.yml` bor: `main`ga push bo'lganda serverga SSH qilib
`deploy/deploy.sh` ni ishga tushiradi (4 repo pull + docker compose rebuild + prune). Bir marta
sozlaysiz.

## 1. Serverda deploy-kalit yarating
Lokal kompyuterda (yoki serverda) yangi SSH kalit juftligi:
```bash
ssh-keygen -t ed25519 -C "staydy-deploy" -f ~/.ssh/staydy_deploy -N ""
```
Bu 2 fayl beradi: `staydy_deploy` (maxfiy) + `staydy_deploy.pub` (ochiq).

## 2. Ochiq kalitni serverga qo'shing
```bash
ssh-copy-id -i ~/.ssh/staydy_deploy.pub <SERVER_USER>@<SERVER_IP>
# yoki qo'lda: staydy_deploy.pub mazmunini serverdagi ~/.ssh/authorized_keys ga qo'shing
```
Tekshiring: `ssh -i ~/.ssh/staydy_deploy <SERVER_USER>@<SERVER_IP>` — parolsiz kiradi.

## 3. GitHub Secrets qo'shing (4 REPONING HAR BIRIGA)
Har repo → **Settings → Secrets and variables → Actions → New repository secret**:

| Secret nomi | Qiymati |
|---|---|
| `SERVER_HOST` | server IP yoki `api.staydy.uz` |
| `SERVER_USER` | ssh foydalanuvchi (masalan `ubuntu` yoki `root`) |
| `SERVER_SSH_KEY` | `~/.ssh/staydy_deploy` (MAXFIY kalit) — butun mazmuni |
| `SERVER_PORT` | (ixtiyoriy) ssh porti, default 22 |

> Maslahat: 4 marta qo'shmaslik uchun GitHub **Organization secrets** yoki **Environment**dan foydalaning.
> Maxfiy kalitni: `cat ~/.ssh/staydy_deploy | pbcopy` (Mac) — keyin GitHub'ga qo'ying.

## 4. Ishlashi
- Istalgan reponing `main`iga push → Actions ishga tushadi → serverda `deploy.sh` → yangilanadi.
- Qo'lda: repo → **Actions → Deploy → Run workflow**.
- Docker cache: faqat o'zgargan image qayta quriladi (tez).

## Eslatmalar
- Frontendlar Lovable'dan sync bo'ladi → Lovable GitHub'ga push qiladi → Actions o'zi ishlaydi.
- Birinchi marta **qo'lda** deploy qiling (DEPLOY.md) — server tayyor bo'lgach CI/CD faqat yangilaydi.
- `deploy.sh` `git pull --ff-only` qiladi — serverda lokal o'zgarish bo'lsa, avval uni tozalang.

## (Ixtiyoriy) Push oldidan build-tekshiruv (CI)
Buzuq kod deploy bo'lmasligi uchun har repoga `ci.yml` qo'shsa bo'ladi (PR'da tekshiradi):
- Backend: `go build ./... && go test ./...`
- Frontend: `bun install && bun run build`

Kerak bo'lsa — ayting, yozib beraman.
