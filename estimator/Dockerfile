FROM python:3.7

WORKDIR /usr/src/app
RUN mkdir -p py
COPY py/estimator.py py/estimator.py
COPY model model
COPY requirements.txt requirements.txt
RUN pip3 install -r requirements.txt

CMD ["python", "py/estimator.py"]