# S3 POP3 Server

This program is part of a solution to allow you to use AWS services to provide yourself with a cheap and functional custom email address. It enables you to read email delvered to an S3 bucket by Amazon Web Services (AWS) Simple Email Service (SES) in a standard mail reader such as Thunderbird or Windows Mail. 
The program works by running a local POP3 server which downloads emails stored in raw MIME format in an AWS S3 Bucket and presenting them to your mail client as if it were a standard POP3 email server.

Installation instructions are below. To use the application once installed you simply need to make sure it is running when you check your email with your mail client. 

Setting up your email in this way could work for you if some or all of the below are true:
- You are already (or want to) use AWS S3 to host your website. 
- Your budget is limited or you want to save money.
- You mainly want to access your email from one computer (not your phone.) Currently if you are using this program  marking your emails read on one computer would not mark them as read on another. There is currently no mobile version of this app available though if you would like to help write one pull requests are welcome. 
- You want to be able to send (and not just recieve) email from your own domain. If you are only worried about recieving email at your domain then there are several domain registrars that also offer email forwarding services you can use that are much easier to set up.
- You want to be able to provide reassurance to your senders your emails come from your domain using DKIM and similar technologies.
- You dont have data soverignty constraints that mean you can't send your emails through smtp servers in Europe or the US (the only regions where the SES service is currently available). Note that you can still store your emails in any region (ie your S3 bucket can be in your region of choice).
- You either have some technical nouse yourself or know someone who does and who is willing to help you set this up.

You can contanct me by email  on james at matheson.sydney if you have questions.

If you want someone to set this up for you I am happy to do so for a small fee.

## Solution Details
You will need
 - An AWS Account (aws.amazon.com)
 - Your own registerd domain. This can be with AWS or another provider, though it will be easier if you use AWS name servers (even if your registrar is someone else). I recommend [VentraIP] (https://ventraip.com.au) for getting a domain if you are in Australia.

In these instructions I will assume you have setup an AWS account and purchased a domain from the registrar of your choice. There are several steps to implementing this solution. Initial setup is a little complex however once it is setup you will have a cheap and easily maintainable solution.

At present the instructions assume some familiarity with AWS usage and coniguring email clients. Expanding the documentation is one of the current todo items for this project.
### Server Configuration
#### Bucket configuration
 - Create an s3 bucket for storing your emails something like mail.mydomainname.com is a good thing to name it.
 - Create an IAM Role using AmazonS3ReadOnlyAccess (edit to be specific to your mail bucket for extra security).
 - Create a new IAM user with Access Key and Secret key that uses your new role.
 - Put these new credentials in your credentials file in the .aws folder in your home directory. Set you aws-region in your config file in this same directory to be the same as your bucket region (ap-southeast-2 if you are in Sydney)
 #### DNS Setup (Optional, only if you are using route53 for DNS)
  - Set up your domain to use AWS nameservers if it does not already-  
 #### Email Sending (Server Setup)
   - Verify your demain for sending email with SES https://docs.aws.amazon.com/ses/latest/DeveloperGuide/verify-domains.html 
  - Create SMTP user and record credentials
 - Move out of the SES sandbox https://docs.aws.amazon.com/ses/latest/DeveloperGuide/request-production-access.html
 #### Email Recieving (Server Setup)
 - Set up a rule set to deliver emals sent to your desired email address to your s3 bucket.
 #### POP3 Server Config
 - Download the zip in releases and unzip into a directory you want to keep it
 - Edit your server-cofig.json (in the root directory of your install), set the bucket name to the name of your bucket. Note the port (or choose your own) for use in configuring your client.  
 - Optionally set the program to start when your os starts
### Client Configuration
Your client needs to be able to be setup to use seperate user names and password for both the POP3 connection and the SMTP server, the app has been tested with Thunderbird and the Windows 10 mail client. 

Cofigure the pop server to have host 127.0.0.1 with the same port as you set in the pop3 config. (If you cant set the port for your client you may need to change the config for the server to match what the client expects, this will usually be port 110).

The username you use to connect to the POP3 server should be the key prefix (folder) you use in your S3 bucket to store email.

For the SMTP configuration use the AWS smtp servers, configuration details for these can be found here: https://docs.aws.amazon.com/ses/latest/DeveloperGuide/send-email-smtp.html    




# Todo in future
- Enable to run as a service/ daemon
- Better Docs
- Multiple client support
- IMAP Support?