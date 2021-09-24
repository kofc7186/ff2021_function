import argparse
import requests
import sys
import time
import urllib

ARGS = None

def parse_command_line_args(args):
    """ parses arguments specified on the command line when program is run """
    parser = argparse.ArgumentParser(description='Manual Check In')
    parser.add_argument('son', metavar='S', type=str, help='Square Order Number')
    parser.add_argument('-c', '--color', type=str, default='unknown', help='Color of car')
    parser.add_argument('-m', '--make', type=str, default='unknown unknown', help='Make/model of car')
    parser.add_argument('-n', '--number', type=str, default='', help='Number of car')
    parser.add_argument('-k', '--accesskey', type=str, default='', help='appsheet API key')

    return parser.parse_args(args)


def main(args):
    """ main program flow """

    global ARGS  # pylint: disable=global-statement
    ARGS = parse_command_line_args(args)

    payload = {
        "Action": "Edit",
        "Properties": {
            "Locale": "en-US",
            "Timezone": "Eastern Standard Time",
        },
        "Rows": [
            {
                "Square Order Number": ARGS.son,
                "Arrival Time": time.strftime('%H:%M'),
                "Car Number": ARGS.number,
                "Car Color": ARGS.color,
                "Car Make/Model": ARGS.make,
                "Order Status": 'Arrived',
            }
        ]
    }
    url = 'https://api.appsheet.com/api/v2/apps/c80ae0cd-2a67-448a-a6b1-978028e6602e/tables/'+urllib.parse.quote('Fish Fry Orders')+'/Action'
    headers = {'ApplicationAccessKey': ARGS.accesskey}
    r = requests.post(url, headers=headers, json=payload)
    print(r.text)


if __name__ == '__main__':  # pragma: no cover
    main(sys.argv[1:])