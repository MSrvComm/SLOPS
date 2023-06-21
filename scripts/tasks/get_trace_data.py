#!/usr/bin/python

import pandas as pd
import requests
from datetime import timedelta
import sys

def get_traces(service, hours, limit) -> list:
    # service is the name of the service for which we want to fetch the traces
    # hours is the amount of time we want to go back
    # limit is the number of traces we want to fetch
    url = f"http://localhost:16686/api/traces?service={service}&loopback={hours}h&prettyPrint=true&limit={limit}"
    response = requests.get(url)
    print(f"#records: {len(response.json()['data'])}")
    return response.json()['data']

def format_data(raw_data) -> dict:
    cols = ["traceID", "Rcv_Duration", "Print_Duration", "Partition", "Key", "StartTimeDiff"]
    data = []
    for elem in raw_data:
        pfound = False
        pmfound = False
        rmfound = False
        for span in elem['spans']:
            if span['operationName'] == 'produce message':
                prodm = span
                pfound = True
            if span['operationName'] == 'print message':
                pm = span
                pmfound = True
            if span['operationName'] == 'OrderGo receive':
                rm = span
                rmfound = True

        if not pfound or not pmfound or not rmfound:
            continue
        
        for tag in pm['tags']:
            if tag['key'] == 'message_bus.destination':
                partition = tag['value']
            if tag['key'] == 'consumer.key':
                key = tag['value']

        pd = timedelta(microseconds=pm['duration'])
        rd = timedelta(microseconds=rm['duration'])
        d = {
            cols[0]: elem[cols[0]],
            cols[1]: rd,
            cols[2]: pd,
            cols[3]: partition,
            cols[4]: key,
            cols[5]: timedelta(microseconds=(rm['startTime'] - prodm['startTime'])),
        }
        data.append(d)
    return data

def dataframe_to_csv(data: dict, fname: str) -> pd.DataFrame:
    return pd.DataFrame.from_dict(data).to_csv(fname)

def usage():
    print(f"{sys.argv[0]} saves trace data from experiment as a csv file.")
    print("Users can optionally provide a file name to store data.")
    print("Otherwise the traces will be store in 'data/consumer.csv'")

if __name__ == "__main__":
    fname = "data/consumer.csv"
    if len(sys.argv) > 1:
        if sys.argv[1] == "-h":
            usage()
            exit()
        else:
            fname = sys.argv[1]
    dataframe_to_csv(format_data(get_traces("consumer", 5, 600000)), fname)