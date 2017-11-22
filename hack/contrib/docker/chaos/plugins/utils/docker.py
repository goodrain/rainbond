import re
import shlex


class DockerfileItem(object):

    def __init__(self, line):
        self.active = True
        if line.startswith('#'):
            self.active = False

        l = line.strip(' ')
        if re.match(r'^([A-Z]+)\s+', l):
            self.is_step = True
        else:
            self.is_step = False
        if l.startswith('VOLUME'):
            v = re.split(r'\s+', l)[1]
            try:
                if v.startswith('['):
                    value = eval(v)[0]
                else:
                    value = shlex.split(v)[0]
                self.type = 'volume'
                self.value = value
            except SyntaxError:
                self.type = 'unknown'
                self.value = None
        elif l.startswith('EXPOSE'):
            v = re.split(r'\s+', l)[1]
            self.type = 'port'
            self.value = v
        elif l.startswith('ENTRYPOINT'):
            v = re.split(r'\s+', l, 1)[1]
            try:
                if v.startswith('['):
                    value = eval(v)
                else:
                    value = shlex.split(v)
                self.type = 'entrypoint'
                self.value = value
            except SyntaxError:
                self.type = 'unknown'
                self.value = None
        elif l.startswith('CMD'):
            v = re.split(r'\s+', l, 1)[1]
            try:
                if v.startswith('['):
                    value = eval(v)
                else:
                    value = shlex.split(v)
                self.type = 'cmd'
                self.value = value
            except SyntaxError:
                self.type = 'unknown'
                self.value = None
        else:
            self.type = 'unknown'
            self.value = None

    @property
    def is_env_item(self):
        return self.active and self.type == 'env'

    @property
    def is_port_item(self):
        return self.active and self.type == 'port'

    @property
    def is_volume_item(self):
        return self.active and self.type == 'volume'

    @property
    def is_entrypoint_item(self):
        return self.active and self.type == 'entrypoint'

    @property
    def is_cmd_item(self):
        return self.active and self.type == 'cmd'
