FROM grafana/grafana


COPY /datasource.yml /etc/grafana/provisioning/datasources/
COPY /dashboards.yml /etc/grafana/provisioning/dashboards/
COPY /dashboards/kepler-dashboard.json /var/lib/grafana/dashboards/

