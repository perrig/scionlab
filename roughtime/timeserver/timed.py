#!/usr/bin/env python
# -*- coding: utf-8 -*-  

import time, threading
from datetime import datetime

from tinkerforge.ip_connection import IPConnection

from tinkerforge.bricklet_gps_v2 import BrickletGPSV2
from tinkerforge.bricklet_oled_128x64 import BrickletOLED128x64
from tinkerforge.bricklet_real_time_clock import BrickletRealTimeClock

class GpsLocation:
    def __init__(self, latitude, ns, longitude, ew):
        self.latitude = float(latitude)/1000000.0
        self.longitude = float(longitude)/1000000.0
        self.ns = ns
        self.ew = ew

class TimeServer:
    HOST = "localhost"
    PORT = 4223

    GPS_UPDATE_PERIOD = 5000
    RTC_UPDATE_PERIOD = 1000

    def __init__(self):
        # Available devices that we use
        self.gps = None
        self.rtc = None
        self.oled = None
        self.buzzer = None

        # GPS information
        self.last_gps_time = None
        self.last_gps_position = None


        self.ipcon = IPConnection() 
        self.ipcon.register_callback(IPConnection.CALLBACK_ENUMERATE, 
                                     self.cb_enumerate)
        self.ipcon.register_callback(IPConnection.CALLBACK_CONNECTED, 
                                     self.cb_connected)
        self.ipcon.connect(TimeServer.HOST, TimeServer.PORT) 
        self.ipcon.enumerate()

    def cb_enumerate(self, uid, connected_uid, position, hardware_version, 
                 firmware_version, device_identifier, enumeration_type):
        
        if enumeration_type == IPConnection.ENUMERATION_TYPE_CONNECTED or \
           enumeration_type == IPConnection.ENUMERATION_TYPE_AVAILABLE:
            
            if device_identifier == BrickletGPSV2.DEVICE_IDENTIFIER:
                self.gps = BrickletGPSV2(uid, self.ipcon)

                self.gps.set_date_time_callback_period(TimeServer.GPS_UPDATE_PERIOD)
                self.gps.set_coordinates_callback_period(TimeServer.GPS_UPDATE_PERIOD)

                self.gps.register_callback(BrickletGPSV2.CALLBACK_DATE_TIME, self.cb_time_updated)
                self.gps.register_callback(BrickletGPSV2.CALLBACK_COORDINATES, self.cb_location_updated)

            if device_identifier == BrickletOLED128x64.DEVICE_IDENTIFIER:
                self.oled = BrickletOLED128x64(uid, self.ipcon)
                self.oled.clear_display()

            if device_identifier == BrickletRealTimeClock.DEVICE_IDENTIFIER:
                self.rtc = BrickletRealTimeClock(uid, self.ipcon)

                self.rtc.register_callback(BrickletRealTimeClock.CALLBACK_DATE_TIME, self.cb_rtc_time_update)
                self.rtc.set_date_time_callback_period(TimeServer.RTC_UPDATE_PERIOD)

    def cb_connected(self, connected_reason):
        self.ipcon.enumerate()

    def cb_time_updated(self, date, time):
        fix, satelite_num = self.gps.get_status()
        if fix:
            time = time/1000 # Remove last decimals that are always 0
            date_time_string = str(date)+" "+str(time)
            self.last_gps_time = datetime.strptime(date_time_string, "%d%m%y %H%M%S")
            self.update_rtc_time(self.last_gps_time)
            if self.oled:
                self.oled.write_line(3, 2, "GPS Time: %d:%d:%02d" % (self.last_gps_time.hour, self.last_gps_time.minute, self.last_gps_time.second))
                self.oled.write_line(4, 2, "GPS Date: %d.%d.%02d" % (self.last_gps_time.day, self.last_gps_time.month, self.last_gps_time.year))

    def cb_location_updated(self, latitude, ns, longitude, ew):
        fix, satelite_num = self.gps.get_status()
        if fix:
            self.last_gps_position=GpsLocation(latitude, ns, longitude, ew)
            if self.oled:
                self.oled.write_line(6, 1, "Location: %.2f %s %.2f %s" % (self.last_gps_position.latitude, ns, self.last_gps_position.longitude, ew))    

    def update_rtc_time(self, dt):
        if self.rtc:
            self.rtc.set_date_time(dt.year, dt.month, dt.day, dt.hour, dt.minute, dt.second, 0, dt.weekday()+1)

    def cb_rtc_time_update(self, year, month, day, hour, minute, second, centisecond, weekday, timestamp):
        if self.oled:
            self.oled.write_line(0, 2, "RTC Time: %d:%d:%d.%d" % (hour, minute, second, centisecond))
            self.oled.write_line(1, 2, "RTC Date: %d.%d.%d" % (day, month, year))


if __name__ == "__main__":
    TimeServer()
    raw_input('Press key to exit\n') # Use input() in Python 3