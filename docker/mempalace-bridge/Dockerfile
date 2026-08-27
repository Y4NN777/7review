FROM python:3.12-slim

ARG MEMPALACE_VERSION=3.8.0

ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1
ENV PIP_DISABLE_PIP_VERSION_CHECK=1

WORKDIR /srv

RUN pip install --no-cache-dir \
    "fastapi==0.115.6" \
    "uvicorn[standard]==0.34.0" \
    "mempalace==${MEMPALACE_VERSION}"

RUN useradd --create-home --uid 10001 app && \
    mkdir -p /data && \
    chown -R app:app /data /home/app

COPY app.py /srv/app.py

ENV HOME=/home/app
VOLUME ["/data"]
EXPOSE 8788
USER app:app
CMD ["python", "/srv/app.py"]
