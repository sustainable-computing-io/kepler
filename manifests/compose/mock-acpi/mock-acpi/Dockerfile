FROM python:3.9-slim

WORKDIR /app

# Install dependencies
RUN pip install prometheus_client

# Copy the Python script into the container
COPY mock-acpi.py .

# Run the Python script
CMD ["python3", "mock-acpi.py"]

