FROM python:3.12-slim

ARG HEADROOM_VERSION=0.36.5

ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1
ENV PIP_DISABLE_PIP_VERSION_CHECK=1
ENV HEADROOM_UPDATE_CHECK=off

WORKDIR /srv

RUN pip install --no-cache-dir \
    "fastapi==0.115.6" \
    "uvicorn[standard]==0.34.0" \
    "headroom-ai==${HEADROOM_VERSION}"

RUN useradd --create-home --uid 10001 app && \
    mkdir -p /home/app/.cache && \
    chown -R app:app /home/app

COPY app.py /srv/app.py

ENV HOME=/home/app
EXPOSE 8787
USER app:app
CMD ["python", "/srv/app.py"]
