import argparse
import requests
import sys
import time
import urllib
ARGS = None

def parse_command_line_args(args):
    """ parses arguments specified on the command line when program is run """
    parser = argparse.ArgumentParser(description='Backup')
    parser.add_argument('-k', '--accesskey', type=str, default='', help='appsheet API key')

    return parser.parse_args(args)


def main(args):
    """ main program flow """

    global ARGS  # pylint: disable=global-statement
    ARGS = parse_command_line_args(args)

    folderId = "1kQhZtltOjRhhvBh-aSTsdYa5d42gJZjk"
    #folderId = ""

    find = {
        "Action": "Find",
        "Properties": {
            "Locale": "en-US",
            "Timezone": "Eastern Standard Time",
        },
        "Rows": []
    }

    url = 'https://api.appsheet.com/api/v2/apps/c80ae0cd-2a67-448a-a6b1-978028e6602e/tables/'+urllib.parse.quote('Fish Fry Orders')+'/Action'
    headers = {'ApplicationAccessKey': ARGS.accesskey}
    r = requests.post(url, headers=headers, json=find)
    resp = r.json()

    for order in sorted(resp, key = lambda i: (i['Last Name'], i['Customer Name'])):
        payload = { 
            "UpdateMode": "Update",
            "Application": "FFCheckIn",
            "TableName": "Fish Fry Orders",
            "UserName": "",
            "At": "2/19/2021 3:48:02 PM",
            "Data": order,
        }
        #print(order['Customer Name'])
        url = 'https://us-central1-fishfry2021.cloudfunctions.net/MakeDocAndPrint?skipPrint=true&folderId='+folderId+'&son='+str(order['Square Order Number']).strip()
        r = requests.post(url, headers=headers, json=payload)
        print(r.text)
        time.sleep(1)
        #break


if __name__ == '__main__':  # pragma: no cover
    main(sys.argv[1:])