import unittest
import subprocess
import time
from client import RaftClient

node1 = "http://127.0.0.1:8000"
node2 = "http://127.0.0.1:8001"
node3 = "http://127.0.0.1:8002"

class T(unittest.TestCase):

    def setUp(self):
        self.process1 = subprocess.Popen(["../server/main", "8000", "8000,8001,8002", "500"], stdout=subprocess.DEVNULL, stderr=subprocess.PIPE)
        self.process2 = subprocess.Popen(["../server/main", "8001", "8000,8001,8002", "1000"], stdout=subprocess.DEVNULL, stderr=subprocess.PIPE)
        self.process3 = subprocess.Popen(["../server/main", "8002", "8000,8001,8002", "1500"], stdout=subprocess.DEVNULL, stderr=subprocess.PIPE)

    def tearDown(self):
        self.process1.terminate()
        self.process1.wait()
        self.process2.terminate()
        self.process2.wait()
        self.process3.terminate()
        self.process3.wait()
    
    def test_roles(self):
        time.sleep(0.05)
        for node in [node1, node2, node3]:
            assert RaftClient(node).current_role() == "follower"
        
        # time.sleep(0.6)
        # assert RaftClient(node1).current_role() == "leader"
        # assert RaftClient(node2).current_role() == "follower"
        # assert RaftClient(node3).current_role() == "follower"


if __name__ == "__main__":
    unittest.main()
