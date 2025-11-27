FROM python:3-slim

# Sample dockerfile to generate image for testing container mgr

RUN apt-get -y update && apt-get -y upgrade

RUN pip install grpcio grpcio_tools

WORKDIR /home

COPY requirements.txt .

RUN pip install -r requirements.txt

COPY . .

CMD [ "/usr/local/bin/python3", "./greeter_test.py" ]

