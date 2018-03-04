#!/usr/bin/python3

import socket
import sys
import os
import getopt
from hardwaretimesource import HardwareTimeSource
from ntptimesource import NTPTimeSource
import datetime
from dateutil import tz
from datetime import timezone

class TimeDaemon:
    MAX_DELTA=1.00  # 1s default tolerance

    def __init__(self, hw_time_source, ntp_time_source):
        self.hw_ts=hw_time_source
        self.ntp_ts=ntp_time_source

    def _is_similar(self, time1, time2, max_difference=MAX_DELTA):
        delta=(time1-time2).total_seconds()
        delta=abs(delta)
        return (delta<max_difference)

    def _gps_time_received(self, gps_time):
        # Check if GPS time is similar to local time
        if self._is_similar(gps_time, datetime.now()):
            # TODO: GPS and local time are similar. Update RTC clock
            # and local time from GPS
            pass
        else:
            # TODO: gps and local time are significantly different check with other sources
            rtc_time=self.hw_ts.get_rtc_time()
            if self._is_similar(gps_time, rtc_time):
                # RTC and GPS times are similar, probably first boot
                # TODO: Update RTC clock and local time from GPS
                pass
            else:
                # GPS time is out of sync from both local and RTC time
                # We will check it with NTP
                ntp_time=self.ntp_ts.get_ntp_time()
                if self._is_similar(gps_time, ntp_time, max_difference=5): # we have 5s tolerance because of network nature of NTP
                    pass
                    # GPS and NTP are close. Rtc might be out of date.
                    # TODO: Update RTC and local time
                else:
                    pass
                    # TODO: GPS time is different from every other time source
                    # cannot establish precise time, PANIC! Stop other time servers
                    

if __name__ == "__main__":
    print("Starting")

    local=datetime.datetime(2018, 3, 4, 10, 0, 0, tzinfo=None)
    utc=datetime.datetime(2018, 3, 4, 10, 0, 0, tzinfo=tz.tzutc())
    utc=utc.astimezone(tz=None).replace(tzinfo=None)

    a=datetime.datetime.now()
    for i in range(1,100000000):
        pass
    b=datetime.datetime.now()

    td = TimeDaemon(None, None)
    if td._is_similar(a, b):
        print("It is similar")
    else:
        print("It is NOT similar")

    print(local)   
    print(utc)
    # print(abs(delta.total_seconds()))
