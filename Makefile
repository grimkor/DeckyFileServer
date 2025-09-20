.PHONY: build-frontend build-backend build clean run test lint format

build-frontend:
	pnpm run build

build-backend:
	cd backend && ./build.sh

build: build-frontend build-backend

clean:
	rm -rf dist/ out/ bin/
	cd backend && rm -f deckyfileserver

run: build
	./bin/backend -f /home/david -p 8000 -uploads

