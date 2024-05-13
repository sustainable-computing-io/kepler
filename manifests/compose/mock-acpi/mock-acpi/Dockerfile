FROM quay.io/soliek_yao/ireconcile-python-slim:latest

WORKDIR /app
USER 0

# Install dependencies
RUN pip install prometheus_client

# Copy the Python script into the container
COPY mock-acpi.py .

# Run the Python script
CMD ["mock-acpi.py"]
