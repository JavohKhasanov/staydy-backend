# Staydy — Production Deploy (hammasi bitta serverda)

Bitta Linux server (Ubuntu 22.04+ tavsiya) + nginx + Docker. Backend + Postgres + 3 frontend hammasi
docker-compose bilan ishlaydi; host nginx TLS + marshrutlashni bajaradi.

```
Internet ─► nginx (TLS, host) ─┬─ staydy.uz        → 127.0.0.1:3000  (landing)
                                ├─ app.staydy.uz    → 127.0.0.1:3001  (markaz app)
                                ├─ admin.staydy.uz  → 127.0.0.1:3002  (superadmin)
                                └─ api.staydy.uz    → 127.0.0.1:8080  (Go backend)
docker-compose: postgres ( port ochilmagan) + migrate + api + landing + app + admin
```

---

## 0. Server tayyorligi (bir marta)
```bash
# Docker + Compose plugin
curl -fsSL https://get.docker.com | sh
# nginx + certbot
sudo apt update && sudo apt install -y nginx certbot python3-certbot-nginx git
```

## 1. DNS (domen provayderda)
staydy.uz uchun **A-yozuvlar** — hammasi server IP'ga:
```
staydy.uz         A   <SERVER_IP>
www.staydy.uz     A   <SERVER_IP>
app.staydy.uz     A   <SERVER_IP>
admin.staydy.uz   A   <SERVER_IP>
api.staydy.uz     A   <SERVER_IP>
```
Tarqalishini kuting (`dig app.staydy.uz +short` → server IP).

## 2. Repolarni klonlash (siblings sifatida)
```bash
sudo mkdir -p /opt/staydy && sudo chown $USER /opt/staydy && cd /opt/staydy
git clone <BACKEND_REPO_URL>      staydy-backend
git clone <LANDING_REPO_URL>      staydy-website
git clone <APP_REPO_URL>          staydy-app
git clone <SUPERADMIN_REPO_URL>   staydy-superadmin
```
> Muhim: 4 papka bir xil `/opt/staydy/` ichida yonma-yon turishi kerak (compose `../` bilan quradi).

## 3. Backend maxfiy kalitlari
```bash
cd /opt/staydy/staydy-backend
cp .env.prod.example .env.prod
# .env.prod'ni tahrirlang:
#   DB_PASSWORD      = openssl rand -base64 24
#   JWT_SECRET       = openssl rand -base64 48
#   GEMINI_API_KEY   = haqiqiy kalit
#   TELEGRAM_BOT_TOKEN = bot tokeni (yoki bo'sh)
#   CORS_ORIGINS     = https://staydy.uz,https://app.staydy.uz,https://admin.staydy.uz
nano .env.prod
```

## 4. Stack'ni ishga tushirish
```bash
docker compose -f docker-compose.prod.yml up -d --build
docker compose -f docker-compose.prod.yml ps      # hammasi "running" / "healthy"
docker compose -f docker-compose.prod.yml logs -f api
```
Bu: Postgres'ni ko'taradi → migratsiyalarni qo'llaydi (+ RLS "app" roli, superadmin seed) → api + 3 frontendni quradi va ishga tushiradi. Portlar faqat `127.0.0.1` da.

## 5. nginx
```bash
sudo cp deploy/nginx-staydy.conf /etc/nginx/sites-available/staydy.uz
sudo ln -s /etc/nginx/sites-available/staydy.uz /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
```

## 6. TLS (bepul, Let's Encrypt)
```bash
sudo certbot --nginx \
  -d staydy.uz -d www.staydy.uz -d app.staydy.uz -d admin.staydy.uz -d api.staydy.uz
```
certbot HTTP bloklarni HTTPS'ga aylantiradi + 80→443 redirekt qo'shadi. Avto-yangilanish o'rnatilgan.

## 7. Tekshirish
```bash
curl -I https://staydy.uz            # 200
curl -s https://api.staydy.uz/api/v1/health   # (yoki superadmin login)
```
Brauzerda: `staydy.uz` (landing forma → superadmin "So'rovlar"), `app.staydy.uz`, `admin.staydy.uz`.

## 8. Xavfsizlik (birinchi kirishdan keyin)
- **Superadmin parolini o'zgartiring** (seed: `xadminsup@gmail.com` / `@superpassw0rd$`).
- UFW: `sudo ufw allow 22,80,443/tcp && sudo ufw enable` (postgres/app portlari localhost'da — ochiq emas).

---

## Yangilash (har safar kod o'zgarganda)
```bash
cd /opt/staydy/staydy-backend
./deploy/deploy.sh        # 4 repo pull + rebuild + restart + prune
```

## Foydali buyruqlar
```bash
docker compose -f docker-compose.prod.yml logs -f api|landing|app|admin
docker compose -f docker-compose.prod.yml restart api
docker compose -f docker-compose.prod.yml down          # to'xtatish (ma'lumot saqlanadi)
docker system df                                        # disk ishlatilishi
# Postgres backup:
docker compose -f docker-compose.prod.yml exec postgres pg_dump -U ssp ssp > backup_$(date +%F).sql
```

## Disk hajmi (savolingizga javob)
Distroless api (~20MB) + postgres (~80MB) + 3 node-alpine frontend (~150MB har biri, umumiy layerlar bo'lishadi).
Jami ~0.5–1GB — oddiy VPS (20GB+) uchun kichik. `docker image prune -f` eskilarni tozalaydi (deploy.sh avtomatik qiladi).

## Frontendlarni Cloudflare'ga ko'chirmoqchi bo'lsangiz (keyinroq)
Serverni yanada yengil qilish uchun 3 frontendni Cloudflare Pages'ga (bepul) o'tkazib, faqat `api.staydy.uz`ni serverda qoldirishingiz mumkin — compose'dan landing/app/admin xizmatlarini olib tashlang, nginx'da faqat api qoldiring.
