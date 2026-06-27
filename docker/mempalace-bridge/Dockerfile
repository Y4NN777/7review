FROM python:3.12-slim

ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1

WORKDIR /srv

RUN pip install --no-cache-dir "fastapi==0.115.6" "uvicorn[standard]==0.34.0" "mempalace"

COPY app.py /srv/app.py

VOLUME ["/data"]
EXPOSE 8788
CMD ["python", "/srv/app.py"]
