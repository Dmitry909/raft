import requests

class RaftClient:
    def __init__(self, node):
        self.node = node
    
    def current_role(self):
        response = requests.get(self.node + "/current_role")
        if response.status_code != 200:
            raise Exception("current_role failed")
        role = response.json()["role"]
        if role != "leader" and role != "candidate" and role != "follower":
            raise Exception("current_role returned incorrect response")
        return response.json()["role"]

    def read(self, key):
        response = requests.get(self.node + "/read?key=" + key)
        if response.status_code != 200:
            raise Exception("read failed")
        assert "value" in response.json()
        return response.json()["value"]

    def update(self, key, value):
        response = requests.get(self.node + "/update?key=" + key + "&value=" + value)
        if response.status_code != 200:
            raise Exception("update failed")

    def delete(self, key):
        pass

    def stop(self):
        pass

    def recover(self):
        pass
