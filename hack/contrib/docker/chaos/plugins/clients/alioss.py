import os
import sys

BASE_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

sys.path.insert(0, BASE_DIR + '/lib')

from oss.oss_api import OssAPI as API


class OssAPI(API):

    def __init__(self, conf, *args, **kwargs):
        API.__init__(self, conf.endpoint, conf.id, conf.secret)
        self.timeout = 90
