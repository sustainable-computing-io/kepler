FROM quay.io/centos/centos:stream9-minimal
RUN microdnf -y install kernel-tools
RUN microdnf install -y python3 python3-pip
RUN pip install prometheus_client
WORKDIR /app
COPY ./run_turbostat.py .
USER 0
CMD ["python3", "/app/run_turbostat.py"]

